//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

// TestBackfillFromLegacyTable verifies existing exam_questions rows are
// converted into frozen versions and attempts are pinned to them.
func TestBackfillFromLegacyTable(t *testing.T) {
	db := newIntegrationDB(t)
	ctx := context.Background()

	// Build a legacy-shaped database: current schema tables plus the old
	// exam_questions table, populated as if migration 0002 never ran.
	if err := db.Exec(`CREATE TABLE exam_questions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		exam_id INTEGER NOT NULL,
		question_id INTEGER NOT NULL,
		score REAL NOT NULL,
		sort_order INTEGER NOT NULL)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	seedQuestion(t, db, 1, "旧题", `"A"`)

	publishedExam := &model.Exam{Title: "已发布", DurationMinutes: 30, TotalScore: 2, Status: constants.ExamPublished}
	draftExam := &model.Exam{Title: "草稿", DurationMinutes: 20, TotalScore: 2, Status: constants.ExamDraft}
	if err := db.Create(publishedExam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(draftExam).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT INTO exam_questions (exam_id, question_id, score, sort_order) VALUES (?, 1, 2, 0)`, publishedExam.ID)
	db.Exec(`INSERT INTO exam_questions (exam_id, question_id, score, sort_order) VALUES (?, 1, 2, 0)`, draftExam.ID)

	oldAttempt := &model.ExamAttempt{
		ExamID: publishedExam.ID, StudentID: 9,
		Status: constants.AttemptSubmitted,
	}
	if err := db.Create(oldAttempt).Error; err != nil {
		t.Fatal(err)
	}
	inProgress := &model.ExamAttempt{
		ExamID: publishedExam.ID, StudentID: 10,
		Status: constants.AttemptInProgress,
	}
	if err := db.Create(inProgress).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	if err := repo.BackfillLegacyVersions(ctx, testSlogLogger(t)); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// Idempotent: a second run changes nothing and must not fail.
	if err := repo.BackfillLegacyVersions(ctx, testSlogLogger(t)); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	v1, err := repo.FindVersionByExamAndNo(ctx, publishedExam.ID, 1)
	if err != nil {
		t.Fatalf("published exam version: %v", err)
	}
	if v1.Status != constants.VersionPublished {
		t.Fatalf("want published, got %s", v1.Status)
	}
	snaps, err := repo.ListVersionQuestions(ctx, v1.ID)
	if err != nil || len(snaps) != 1 || snaps[0].Content != "旧题" {
		t.Fatalf("snapshots wrong: %v %+v", err, snaps)
	}

	// Exam points at v1.
	gotExam, _ := repo.FindExamByID(ctx, publishedExam.ID)
	if gotExam.CurrentVersionID != v1.ID {
		t.Fatalf("current_version_id = %d, want %d", gotExam.CurrentVersionID, v1.ID)
	}

	// Both attempts pinned.
	var attempts []model.ExamAttempt
	db.Where("exam_id = ?", publishedExam.ID).Find(&attempts)
	for _, a := range attempts {
		if a.VersionID != v1.ID {
			t.Fatalf("attempt %d version_id = %d, want %d", a.ID, a.VersionID, v1.ID)
		}
	}

	// Draft exam gets a draft version, not published.
	dv, err := repo.FindVersionByExamAndNo(ctx, draftExam.ID, 1)
	if err != nil {
		t.Fatalf("draft exam version: %v", err)
	}
	if dv.Status != constants.VersionDraft {
		t.Fatalf("want draft, got %s", dv.Status)
	}
	draftGot, _ := repo.FindExamByID(ctx, draftExam.ID)
	if draftGot.CurrentVersionID != 0 {
		t.Fatalf("draft exam should not point at a version, got %d", draftGot.CurrentVersionID)
	}
}
