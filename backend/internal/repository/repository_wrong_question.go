package repository

import (
	"context"
	"fmt"

	"github.com/gbexam/online-exam/internal/model"
)

// UpsertWrongQuestion records a wrong answer, incrementing the wrong count.
func (r *Repository) UpsertWrongQuestion(ctx context.Context, w *model.WrongQuestion) error {
	var existing model.WrongQuestion
	err := r.db.WithContext(ctx).Where("student_id = ? AND question_id = ?", w.StudentID, w.QuestionID).First(&existing).Error
	if err == nil {
		res := r.db.WithContext(ctx).Model(&model.WrongQuestion{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"wrong_count":   existing.WrongCount + 1,
			"last_wrong_at": w.LastWrongAt,
			"status":        "unresolved",
		})
		if res.Error != nil {
			return fmt.Errorf("update wrong question: %w", res.Error)
		}
		w.ID = existing.ID
		return nil
	}
	if isRecordNotFound(err) {
		if createErr := r.db.WithContext(ctx).Create(w).Error; createErr != nil {
			return fmt.Errorf("create wrong question: %w", createErr)
		}
		return nil
	}
	return fmt.Errorf("find wrong question: %w", err)
}

// ListWrongQuestions returns a page of a student's wrong questions.
func (r *Repository) ListWrongQuestions(ctx context.Context, studentID uint, knowledgePoint string, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.WrongQuestion{}).Where("student_id = ?", studentID)
	if knowledgePoint != "" {
		q = q.Where("knowledge_point = ?", knowledgePoint)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count wrong questions: %w", err)
	}
	var items []model.WrongQuestion
	p, ps := NormalizePage(page, pageSize)
	if err := q.Order("last_wrong_at DESC").Limit(ps).Offset((p - 1) * ps).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list wrong questions: %w", err)
	}
	return items, total, nil
}

// DeleteWrongQuestion removes one wrong question record.
func (r *Repository) DeleteWrongQuestion(ctx context.Context, id, studentID uint) error {
	res := r.db.WithContext(ctx).Where("id = ? AND student_id = ?", id, studentID).Delete(&model.WrongQuestion{})
	if res.Error != nil {
		return fmt.Errorf("delete wrong question: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkWrongQuestionResolved updates the status of a wrong question.
func (r *Repository) MarkWrongQuestionResolved(ctx context.Context, id, studentID uint) error {
	res := r.db.WithContext(ctx).Model(&model.WrongQuestion{}).Where("id = ? AND student_id = ?", id, studentID).Update("status", "resolved")
	if res.Error != nil {
		return fmt.Errorf("resolve wrong question: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
