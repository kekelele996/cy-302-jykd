package repository

import (
	"context"
	"fmt"

	"github.com/gbexam/online-exam/internal/model"
)

// CreateAttempt inserts an exam attempt.
func (r *Repository) CreateAttempt(ctx context.Context, a *model.ExamAttempt) error {
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	return nil
}

// FindAttemptByID returns an attempt.
func (r *Repository) FindAttemptByID(ctx context.Context, id uint) (*model.ExamAttempt, error) {
	var a model.ExamAttempt
	err := r.db.WithContext(ctx).First(&a, id).Error
	if err != nil {
		return nil, wrapQuery("find attempt by id", err)
	}
	return &a, nil
}

// UpdateAttempt updates attempt status and scores.
func (r *Repository) UpdateAttempt(ctx context.Context, a *model.ExamAttempt) error {
	res := r.db.WithContext(ctx).Model(&model.ExamAttempt{}).Where("id = ?", a.ID).Updates(map[string]any{
		"status":          a.Status,
		"submitted_at":    a.SubmittedAt,
		"deadline":        a.Deadline,
		"objective_score": a.ObjectiveScore,
		"total_score":     a.TotalScore,
	})
	if res.Error != nil {
		return fmt.Errorf("update attempt: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindInProgressAttempt returns the student's current unfinished attempt for an exam.
func (r *Repository) FindInProgressAttempt(ctx context.Context, examID, studentID uint) (*model.ExamAttempt, error) {
	var a model.ExamAttempt
	err := r.db.WithContext(ctx).
		Where("exam_id = ? AND student_id = ? AND status = ?", examID, studentID, "in_progress").
		Order("id DESC").First(&a).Error
	if err != nil {
		return nil, wrapQuery("find in progress attempt", err)
	}
	return &a, nil
}

// ListAttemptsByStudent returns a page of attempts for a student.
func (r *Repository) ListAttemptsByStudent(ctx context.Context, studentID, examID uint, page, pageSize int) ([]model.ExamAttempt, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ExamAttempt{}).Where("student_id = ?", studentID)
	if examID != 0 {
		q = q.Where("exam_id = ?", examID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count attempts: %w", err)
	}
	var items []model.ExamAttempt
	p, ps := NormalizePage(page, pageSize)
	if err := q.Order("id DESC").Limit(ps).Offset((p - 1) * ps).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list attempts: %w", err)
	}
	return items, total, nil
}

// ListAttemptsByExam returns all attempts for an exam.
func (r *Repository) ListAttemptsByExam(ctx context.Context, examID uint) ([]model.ExamAttempt, error) {
	var items []model.ExamAttempt
	if err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list attempts by exam: %w", err)
	}
	return items, nil
}

// CountAttempts returns the total number of attempts.
func (r *Repository) CountAttempts(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.ExamAttempt{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count attempts: %w", err)
	}
	return total, nil
}
