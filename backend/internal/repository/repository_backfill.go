package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

// BackfillPaperVersions freezes one paper version for exams/attempts that
// predate versioning. It is idempotent: exams that already point at a version
// and attempts already bound to one are skipped.
//
// Existing exam_questions rows reference live questions; the backfill copies
// whatever the question bank currently holds, which matches the pre-freeze
// behavior those records had, and afterwards regrouping can never break them.
func (r *Repository) BackfillPaperVersions(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var examIDs []uint
		if err := tx.Model(&model.ExamAttempt{}).
			Distinct("exam_id").
			Where("paper_version_id = 0").
			Scan(&examIDs).Error; err != nil {
			return fmt.Errorf("list exams needing backfill: %w", err)
		}
		var publishedIDs []uint
		if err := tx.Model(&model.Exam{}).
			Where("current_version_id = 0 AND status <> ?", constants.ExamDraft).
			Pluck("id", &publishedIDs).Error; err != nil {
			return fmt.Errorf("list published exams needing backfill: %w", err)
		}
		examIDs = append(examIDs, publishedIDs...)
		seen := make(map[uint]bool, len(examIDs))
		for _, id := range examIDs {
			if id == 0 || seen[id] {
				continue
			}
			seen[id] = true
			if err := r.backfillOneExam(ctx, tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) backfillOneExam(ctx context.Context, tx *gorm.DB, examID uint) error {
	var exam model.Exam
	if err := tx.First(&exam, examID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("backfill load exam %d: %w", examID, err)
	}

	versionID := exam.CurrentVersionID
	if versionID == 0 {
		var items []model.ExamQuestion
		if err := tx.Where("exam_id = ?", examID).Order("sort_order ASC").Find(&items).Error; err != nil {
			return fmt.Errorf("backfill list exam questions: %w", err)
		}
		questionIDs := make([]uint, 0, len(items))
		for _, it := range items {
			questionIDs = append(questionIDs, it.QuestionID)
		}
		var questions []model.Question
		if len(questionIDs) > 0 {
			if err := tx.Where("id IN ?", questionIDs).Find(&questions).Error; err != nil {
				return fmt.Errorf("backfill list questions: %w", err)
			}
		}
		questionMap := make(map[uint]model.Question, len(questions))
		for _, q := range questions {
			questionMap[q.ID] = q
		}

		version := model.PaperVersion{
			ExamID:     exam.ID,
			VersionNo:  1,
			Title:      exam.Title,
			TotalScore: exam.TotalScore,
			CreatedBy:  exam.CreatedBy,
			SnapshotAt: time.Now(),
		}
		if err := tx.Create(&version).Error; err != nil {
			return fmt.Errorf("backfill create version: %w", err)
		}
		frozen := make([]model.PaperVersionQuestion, 0, len(items))
		for _, it := range items {
			q, ok := questionMap[it.QuestionID]
			if !ok {
				// Source question was deleted before versioning existed; keep a
				// placeholder so historical attempts still resolve to a row.
				q = model.Question{ID: it.QuestionID, Content: "（原题目已删除）"}
			}
			frozen = append(frozen, model.PaperVersionQuestion{
				VersionID:      version.ID,
				QuestionID:     q.ID,
				Type:           q.Type,
				Content:        q.Content,
				Options:        q.Options,
				Answer:         q.Answer,
				Analysis:       q.Analysis,
				Difficulty:     q.Difficulty,
				KnowledgePoint: q.KnowledgePoint,
				Score:          it.Score,
				SortOrder:      it.SortOrder,
			})
		}
		if len(frozen) > 0 {
			if err := tx.Create(&frozen).Error; err != nil {
				return fmt.Errorf("backfill create frozen questions: %w", err)
			}
		}
		versionID = version.ID
		if err := tx.Model(&model.Exam{}).Where("id = ?", exam.ID).
			Update("current_version_id", versionID).Error; err != nil {
			return fmt.Errorf("backfill point exam version: %w", err)
		}
	}

	res := tx.Model(&model.ExamAttempt{}).
		Where("exam_id = ? AND paper_version_id = 0", examID).
		Update("paper_version_id", versionID)
	if res.Error != nil {
		return fmt.Errorf("backfill bind attempts: %w", res.Error)
	}
	return nil
}
