package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

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

// DeleteExam removes an exam and all of its paper versions (only creator/admin).
func (r *Repository) DeleteExam(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var versionIDs []uint
		if err := tx.Model(&model.PaperVersion{}).
			Where("exam_id = ?", id).Pluck("id", &versionIDs).Error; err != nil {
			return fmt.Errorf("list paper versions for delete: %w", err)
		}
		if len(versionIDs) > 0 {
			if err := tx.Where("version_id IN ?", versionIDs).
				Delete(&model.PaperVersionQuestion{}).Error; err != nil {
				return fmt.Errorf("delete paper version questions: %w", err)
			}
			if err := tx.Where("exam_id = ?", id).
				Delete(&model.PaperVersion{}).Error; err != nil {
				return fmt.Errorf("delete paper versions: %w", err)
			}
		}
		if err := tx.Where("exam_id = ?", id).Delete(&model.ExamQuestion{}).Error; err != nil {
			return fmt.Errorf("delete exam questions: %w", err)
		}
		res := tx.Delete(&model.Exam{}, id)
		if res.Error != nil {
			return fmt.Errorf("delete exam: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
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

// ReplaceExamQuestions atomically replaces the paper questions for an exam.
func (r *Repository) ReplaceExamQuestions(ctx context.Context, examID uint, items []model.ExamQuestion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("exam_id = ?", examID).Delete(&model.ExamQuestion{}).Error; err != nil {
			return fmt.Errorf("delete old exam questions: %w", err)
		}
		if len(items) == 0 {
			return nil
		}
		if err := tx.Create(&items).Error; err != nil {
			return fmt.Errorf("create exam questions: %w", err)
		}
		return nil
	})
}

// ListExamQuestions returns paper questions ordered by sort order.
func (r *Repository) ListExamQuestions(ctx context.Context, examID uint) ([]model.ExamQuestion, error) {
	var items []model.ExamQuestion
	if err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Order("sort_order ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list exam questions: %w", err)
	}
	return items, nil
}

// ListExamQuestionsTx is ListExamQuestions callable inside a transaction, so
// reads see uncommitted rows written earlier in the same transaction.
func (r *Repository) ListExamQuestionsTx(ctx context.Context, tx *gorm.DB, examID uint) ([]model.ExamQuestion, error) {
	var items []model.ExamQuestion
	if err := tx.WithContext(ctx).Where("exam_id = ?", examID).Order("sort_order ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list exam questions tx: %w", err)
	}
	return items, nil
}

// CountExamQuestionsTx is CountExamQuestions callable inside a transaction.
func (r *Repository) CountExamQuestionsTx(ctx context.Context, tx *gorm.DB, examID uint) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Model(&model.ExamQuestion{}).Where("exam_id = ?", examID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count exam questions tx: %w", err)
	}
	return count, nil
}

// CountExamQuestions returns the number of questions in a paper.
func (r *Repository) CountExamQuestions(ctx context.Context, examID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ExamQuestion{}).Where("exam_id = ?", examID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count exam questions: %w", err)
	}
	return count, nil
}

// CountExams returns the total number of exams.
func (r *Repository) CountExams(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Exam{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count exams: %w", err)
	}
	return total, nil
}

// ReplaceExamQuestionsTx is ReplaceExamQuestions callable inside a transaction.
func (r *Repository) ReplaceExamQuestionsTx(ctx context.Context, tx *gorm.DB, examID uint, items []model.ExamQuestion) error {
	if err := tx.WithContext(ctx).Where("exam_id = ?", examID).Delete(&model.ExamQuestion{}).Error; err != nil {
		return fmt.Errorf("delete old exam questions: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	if err := tx.WithContext(ctx).Create(&items).Error; err != nil {
		return fmt.Errorf("create exam questions: %w", err)
	}
	return nil
}

// UpdateExamTotalScoreTx updates cached exam totals inside a transaction.
func (r *Repository) UpdateExamTotalScoreTx(ctx context.Context, tx *gorm.DB, examID uint, totalScore float64) error {
	res := tx.WithContext(ctx).Model(&model.Exam{}).
		Where("id = ?", examID).
		Update("total_score", totalScore)
	if res.Error != nil {
		return fmt.Errorf("update exam total score: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishExamIfDraft atomically flips draft -> published. RowsAffected is 0
// when the exam is already published/closed (duplicate/concurrent publish),
// letting the service treat that case idempotently instead of overwriting.
func (r *Repository) PublishExamIfDraft(ctx context.Context, tx *gorm.DB, id uint, versionID uint) (int64, error) {
	res := tx.WithContext(ctx).Model(&model.Exam{}).
		Where("id = ? AND status = ?", id, "draft").
		Updates(map[string]any{
			"status":             "published",
			"current_version_id": versionID,
			"revision":           gorm.Expr("revision + 1"),
		})
	if res.Error != nil {
		return 0, fmt.Errorf("publish exam if draft: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// FindQuestionsByIDsTx is FindQuestionsByIDs callable inside a transaction and
// locks the question rows, so a concurrent question edit cannot interleave with
// the snapshot copy.
func (r *Repository) FindQuestionsByIDsTx(ctx context.Context, tx *gorm.DB, ids []uint) (map[uint]model.Question, error) {
	result := make(map[uint]model.Question, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var questions []model.Question
	if err := tx.WithContext(ctx).
		Clauses(clauseForUpdate()).
		Where("id IN ?", ids).Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("find questions by ids tx: %w", err)
	}
	for _, q := range questions {
		result[q.ID] = q
	}
	return result, nil
}
