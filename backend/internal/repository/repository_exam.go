package repository

import (
	"context"
	"fmt"

	"github.com/gbexam/online-exam/internal/model"
)

// ExamFilter holds optional filters for exam list queries.
type ExamFilter struct {
	Status    string
	Keyword   string
	CreatedBy uint
}

// CreateExam inserts an exam.
func (r *Repository) CreateExam(ctx context.Context, exam *model.Exam) error {
	if err := r.db.WithContext(ctx).Create(exam).Error; err != nil {
		return fmt.Errorf("create exam: %w", err)
	}
	return nil
}

// FindExamByID returns an exam.
func (r *Repository) FindExamByID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	err := r.db.WithContext(ctx).First(&exam, id).Error
	if err != nil {
		return nil, wrapQuery("find exam by id", err)
	}
	return &exam, nil
}

// UpdateExam updates exam metadata.
func (r *Repository) UpdateExam(ctx context.Context, exam *model.Exam) error {
	res := r.db.WithContext(ctx).Model(&model.Exam{}).Where("id = ?", exam.ID).Updates(map[string]any{
		"title":            exam.Title,
		"description":      exam.Description,
		"duration_minutes": exam.DurationMinutes,
		"total_score":      exam.TotalScore,
		"start_time":       exam.StartTime,
		"end_time":         exam.EndTime,
		"status":           exam.Status,
	})
	if res.Error != nil {
		return fmt.Errorf("update exam: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteExam removes the exam row. Frozen versions, attempts and answers are
// deliberately retained, so historical result reviews still render the exact
// paper used at composition time.
func (r *Repository) DeleteExam(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&model.Exam{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete exam: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListExams returns a filtered page of exams.
func (r *Repository) ListExams(ctx context.Context, filter ExamFilter, page, pageSize int) ([]model.Exam, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Exam{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Keyword != "" {
		q = q.Where("title LIKE ?", "%"+filter.Keyword+"%")
	}
	if filter.CreatedBy != 0 {
		q = q.Where("created_by = ?", filter.CreatedBy)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count exams: %w", err)
	}

	var exams []model.Exam
	p, ps := NormalizePage(page, pageSize)
	if err := q.Order("id DESC").Limit(ps).Offset((p - 1) * ps).Find(&exams).Error; err != nil {
		return nil, 0, fmt.Errorf("list exams: %w", err)
	}
	return exams, total, nil
}

// CountExams returns the total number of exams.
func (r *Repository) CountExams(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Exam{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count exams: %w", err)
	}
	return total, nil
}

// CountExams returns the total number of exams.
