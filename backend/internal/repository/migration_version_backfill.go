package repository

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

// legacyExamQuestion mirrors the pre-versioning exam_questions table.
type legacyExamQuestion struct {
	ID         uint    `gorm:"column:id;primaryKey"`
	ExamID     uint    `gorm:"column:exam_id"`
	QuestionID uint    `gorm:"column:question_id"`
	Score      float64 `gorm:"column:score"`
	SortOrder  int     `gorm:"column:sort_order"`
}

func (legacyExamQuestion) TableName() string { return "exam_questions" }

// BackfillLegacyVersions performs the one-time migration from the legacy
// exam_questions table to immutable exam versions and snapshots.
//
// Every exam without a version receives one version (published if the exam is
// published/closed, otherwise draft) whose snapshots copy the question bank
// content as it currently stands. Existing attempts are pinned to that
// version. It is idempotent: exams that already have versions are skipped.
// A fresh database (no legacy table) is a no-op.
func (r *Repository) BackfillLegacyVersions(ctx context.Context, logger *slog.Logger) error {
	db := r.db.WithContext(ctx)

	if !db.Migrator().HasTable("exam_questions") {
		return nil
	}

	var versionCount int64
	if err := db.Model(&model.ExamVersion{}).Count(&versionCount).Error; err != nil {
		return fmt.Errorf("count versions during backfill: %w", err)
	}
	if versionCount > 0 {
		return nil
	}

	var legacyCount int64
	if err := db.Table("exam_questions").Count(&legacyCount).Error; err != nil {
		return fmt.Errorf("count legacy exam questions: %w", err)
	}
	if legacyCount == 0 {
		return nil
	}

	var exams []model.Exam
	if err := db.Find(&exams).Error; err != nil {
		return fmt.Errorf("list exams during backfill: %w", err)
	}

	for _, exam := range exams {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return backfillExamTx(tx, exam)
		}); err != nil {
			return fmt.Errorf("backfill exam %d: %w", exam.ID, err)
		}
	}
	logger.Info("legacy exam questions backfilled into frozen versions", "exams", len(exams))
	return nil
}

func backfillExamTx(tx *gorm.DB, exam model.Exam) error {
	var existing int64
	if err := tx.Model(&model.ExamVersion{}).Where("exam_id = ?", exam.ID).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	var legacy []legacyExamQuestion
	if err := tx.Where("exam_id = ?", exam.ID).Order("sort_order ASC").Find(&legacy).Error; err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}

	questionIDs := make([]uint, 0, len(legacy))
	for _, lq := range legacy {
		questionIDs = append(questionIDs, lq.QuestionID)
	}
	var questions []model.Question
	if err := tx.Unscoped().Where("id IN ?", questionIDs).Find(&questions).Error; err != nil {
		return err
	}
	questionMap := make(map[uint]model.Question, len(questions))
	for _, q := range questions {
		questionMap[q.ID] = q
	}

	status := constants.VersionDraft
	if exam.Status == constants.ExamPublished || exam.Status == constants.ExamClosed {
		status = constants.VersionPublished
	}
	version := &model.ExamVersion{
		ExamID:          exam.ID,
		VersionNo:       1,
		Status:          status,
		TotalScore:      exam.TotalScore,
		DurationMinutes: exam.DurationMinutes,
		CreatedBy:       exam.CreatedBy,
	}
	if err := tx.Create(version).Error; err != nil {
		return err
	}

	snapshots := make([]model.ExamVersionQuestion, 0, len(legacy))
	total := 0.0
	for _, lq := range legacy {
		q, ok := questionMap[lq.QuestionID]
		if !ok {
			// Source question already gone even from the legacy table; keep a
			// minimal placeholder so attempt review still renders structure.
			q = model.Question{ID: lq.QuestionID, Content: "（题目已撤回）"}
		}
		snapshots = append(snapshots, model.ExamVersionQuestion{
			VersionID:      version.ID,
			ExamID:         exam.ID,
			QuestionID:     q.ID,
			SortOrder:      lq.SortOrder,
			Score:          lq.Score,
			Type:           q.Type,
			Content:        q.Content,
			Options:        q.Options,
			Answer:         q.Answer,
			Analysis:       q.Analysis,
			Difficulty:     q.Difficulty,
			KnowledgePoint: q.KnowledgePoint,
		})
		total += lq.Score
	}
	if err := tx.Create(&snapshots).Error; err != nil {
		return err
	}
	version.TotalScore = total

	updates := map[string]any{"current_version_id": 0}
	if status == constants.VersionPublished {
		updates["current_version_id"] = version.ID
		updates["total_score"] = total
	}
	if err := tx.Model(&model.Exam{}).Where("id = ?", exam.ID).Updates(updates).Error; err != nil {
		return err
	}

	// Pin historical and in-progress attempts to the frozen version.
	return tx.Model(&model.ExamAttempt{}).
		Where("exam_id = ? AND (version_id = 0 OR version_id IS NULL)", exam.ID).
		Update("version_id", version.ID).Error
}
