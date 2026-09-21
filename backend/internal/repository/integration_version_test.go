//go:build integration

package repository

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

func newIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedQuestion(t *testing.T, db *gorm.DB, id uint, content, answer string) {
	t.Helper()
	q := model.Question{
		ID:             id,
		Type:           constants.QuestionSingle,
		Content:        content,
		Options:        `[{"key":"A","text":"甲"},{"key":"B","text":"乙"}]`,
		Answer:         answer,
		Difficulty:     constants.DifficultyEasy,
		KnowledgePoint: "kp",
		Score:          2,
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
}

// TestVersionFreezeEndToEnd exercises the full frozen-paper lifecycle against
// a real database.
func TestVersionFreezeEndToEnd(t *testing.T) {
	db := newIntegrationDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	seedQuestion(t, db, 1, "原始题干", `"A"`)
	seedQuestion(t, db, 2, "第二题", `"A"`)

	// 1. Create exam + draft version with snapshots in one transaction.
	exam := &model.Exam{Title: "E2E", DurationMinutes: 60, Status: constants.ExamDraft, CreatedBy: 7}
	version, err := repo.CreateExamWithDraftVersion(ctx, exam, func() ([]model.ExamVersionQuestion, float64, int, error) {
		var qs []model.Question
		db.Order("id ASC").Find(&qs)
		items := make([]model.ExamVersionQuestion, 0, len(qs))
		for i, q := range qs {
			items = append(items, model.ExamVersionQuestion{
				QuestionID: q.ID, SortOrder: i, Score: 2,
				Type: q.Type, Content: q.Content, Options: q.Options, Answer: q.Answer,
				Difficulty: q.Difficulty, KnowledgePoint: q.KnowledgePoint,
			})
		}
		return items, 4, 60, nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if version.VersionNo != 1 || exam.ID == 0 {
		t.Fatalf("bad version/exam ids: %+v %+v", version, exam)
	}

	// 2. Publish freezes v1.
	published, err := repo.PublishVersion(ctx, exam.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != constants.VersionPublished {
		t.Fatalf("status = %s", published.Status)
	}

	// 3. Repeated publish takes effect only once.
	if _, err := repo.PublishVersion(ctx, exam.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("repeat publish err = %v, want ErrConflict", err)
	}

	// 4. Attempt binds v1.
	attempt := &model.ExamAttempt{
		ExamID: exam.ID, VersionID: published.ID, StudentID: 42,
		Status: constants.AttemptInProgress, StartedAt: time.Now(), Deadline: time.Now().Add(time.Hour),
	}
	if err := repo.CreateAttempt(ctx, attempt); err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	// 5. Mutate AND withdraw the source question after the attempt started.
	db.Model(&model.Question{}).Where("id = 1").Updates(map[string]any{
		"content": "篡改后的题干", "answer": `"B"`,
	})
	if err := db.Delete(&model.Question{}, 1).Error; err != nil {
		t.Fatalf("soft delete question: %v", err)
	}

	// 6. The in-progress attempt still renders the frozen content.
	snaps, err := repo.ListVersionQuestions(ctx, attempt.VersionID)
	if err != nil {
		t.Fatalf("list snaps: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	found := false
	for _, s := range snaps {
		if s.QuestionID == 1 {
			found = true
			if s.Content != "原始题干" || s.Answer != `"A"` {
				t.Fatalf("snapshot mutated: %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("withdrawn question missing from frozen paper")
	}

	// 7. Refreshing/re-reading returns identical data (same version, same rows).
	snaps2, _ := repo.ListVersionQuestions(ctx, attempt.VersionID)
	if len(snaps2) != len(snaps) || snaps2[0].Content != snaps[0].Content {
		t.Fatal("paper content differs across reads")
	}

	// 8. Concurrent submit CAS: only first wins.
	if err := repo.SubmitAttemptCAS(ctx, attempt.ID, time.Now(), 4, 4); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if err := repo.SubmitAttemptCAS(ctx, attempt.ID, time.Now(), 0, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("second submit err = %v, want ErrConflict", err)
	}

	// 9. Regenerate after publish creates a new draft; v1 stays published.
	v2, err := repo.SaveDraftVersion(ctx, exam.ID, func() ([]model.ExamVersionQuestion, float64, int, error) {
		items := []model.ExamVersionQuestion{{
			QuestionID: 2, SortOrder: 0, Score: 5,
			Type: constants.QuestionSingle, Content: "第二题", Answer: `"A"`,
			Difficulty: constants.DifficultyEasy, KnowledgePoint: "kp",
		}}
		return items, 5, 45, nil
	})
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if v2.VersionNo != 2 || v2.Status != constants.VersionDraft {
		t.Fatalf("v2 = %+v", v2)
	}
	// Second concurrent regeneration is blocked by pending draft.
	if _, err := repo.SaveDraftVersion(ctx, exam.ID, func() ([]model.ExamVersionQuestion, float64, int, error) {
		return nil, 0, 0, nil
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent regenerate err = %v, want ErrConflict", err)
	}
	// Old version still readable.
	old, err := repo.FindVersionByExamAndNo(ctx, exam.ID, 1)
	if err != nil {
		t.Fatalf("find v1: %v", err)
	}
	oldSnaps, err := repo.ListVersionQuestions(ctx, old.ID)
	if err != nil || len(oldSnaps) != 2 {
		t.Fatalf("old version snapshots: %v %d", err, len(oldSnaps))
	}

	// 10. Publishing v2 archives v1 and moves the exam pointer; historical
	// attempt still resolves v1.
	published2, err := repo.PublishVersion(ctx, exam.ID)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	current, err := repo.FindCurrentVersion(ctx, exam.ID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.ID != published2.ID {
		t.Fatalf("current = %d, want %d", current.ID, published2.ID)
	}
	v1After, _ := repo.FindVersionByID(ctx, old.ID)
	if v1After.Status != constants.VersionArchived {
		t.Fatalf("v1 status = %s, want archived", v1After.Status)
	}
	historical, _ := repo.ListVersionQuestions(ctx, attempt.VersionID)
	if len(historical) != 2 || historical[0].Content != "原始题干" {
		t.Fatal("historical attempt paper not preserved after new version publish")
	}

	// 11. Deleting the exam keeps versions and answers for historical review.
	if err := repo.DeleteExam(ctx, exam.ID); err != nil {
		t.Fatalf("delete exam: %v", err)
	}
	if _, err := repo.FindVersionByID(ctx, attempt.VersionID); err != nil {
		t.Fatalf("version must survive exam deletion: %v", err)
	}

	// Backfill on the versioned database must be a no-op.
	if err := repo.BackfillLegacyVersions(ctx, logger); err != nil {
		t.Fatalf("backfill: %v", err)
	}
}

// TestPublishAndRegenerateAreSerialized verifies the exam row lock prevents
// lost updates under concurrent publish/regenerate pressure.
func TestPublishAndRegenerateAreSerialized(t *testing.T) {
	db := newIntegrationDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	seedQuestion(t, db, 1, "题", `"A"`)

	exam := &model.Exam{Title: "race", DurationMinutes: 60, Status: constants.ExamDraft}
	if _, err := repo.CreateExamWithDraftVersion(ctx, exam, func() ([]model.ExamVersionQuestion, float64, int, error) {
		return []model.ExamVersionQuestion{{QuestionID: 1, SortOrder: 0, Score: 2, Type: constants.QuestionSingle, Content: "题", Answer: `"A"`, Difficulty: "easy", KnowledgePoint: "kp"}}, 2, 60, nil
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	var pubOK, pubConflict int
	var pubMu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.PublishVersion(ctx, exam.ID); err != nil {
				pubMu.Lock()
				pubConflict++
				pubMu.Unlock()
			} else {
				pubMu.Lock()
				pubOK++
				pubMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if pubOK != 1 {
		t.Fatalf("publish successes = %d, want exactly 1 (conflicts=%d)", pubOK, pubConflict)
	}
}
