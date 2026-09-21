package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/model"
)

// QuestionFilter holds optional filters for question list queries.
type QuestionFilter struct {
	Type           string
	Difficulty     string
	KnowledgePoint string
	Keyword        string
}

// CreateQuestion inserts a question.
func (r *Repository) CreateQuestion(ctx context.Context, q *model.Question) error {
	if err := r.db.WithContext(ctx).Create(q).Error; err != nil {
		return fmt.Errorf("create question: %w", err)
	}
	return nil
}

// CreateQuestionsBatch inserts multiple questions in one transaction.
func (r *Repository) CreateQuestionsBatch(ctx context.Context, questions []model.Question) error {
	if len(questions) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&questions).Error; err != nil {
			return fmt.Errorf("create questions batch: %w", err)
		}
		return nil
	})
}

// FindQuestionByID returns a question.
func (r *Repository) FindQuestionByID(ctx context.Context, id uint) (*model.Question, error) {
	var q model.Question
	err := r.db.WithContext(ctx).First(&q, id).Error
	if err != nil {
		return nil, wrapQuery("find question by id", err)
	}
	return &q, nil
}

// UpdateQuestion updates a question by id.
func (r *Repository) UpdateQuestion(ctx context.Context, q *model.Question) error {
	res := r.db.WithContext(ctx).Model(&model.Question{}).Where("id = ?", q.ID).Updates(map[string]any{
		"type":            q.Type,
		"content":         q.Content,
		"options":         q.Options,
		"answer":          q.Answer,
		"analysis":        q.Analysis,
		"difficulty":      q.Difficulty,
		"knowledge_point": q.KnowledgePoint,
		"score":           q.Score,
	})
	if res.Error != nil {
		return fmt.Errorf("update question: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteQuestion removes a question.
func (r *Repository) DeleteQuestion(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&model.Question{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete question: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListQuestions returns a filtered page of questions.
func (r *Repository) ListQuestions(ctx context.Context, filter QuestionFilter, page, pageSize int) ([]model.Question, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Question{})
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Difficulty != "" {
		q = q.Where("difficulty = ?", filter.Difficulty)
	}
	if filter.KnowledgePoint != "" {
		q = q.Where("knowledge_point = ?", filter.KnowledgePoint)
	}
	if filter.Keyword != "" {
		q = q.Where("content LIKE ?", "%"+filter.Keyword+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count questions: %w", err)
	}

	var questions []model.Question
	p, ps := NormalizePage(page, pageSize)
	if err := q.Order("id DESC").Limit(ps).Offset((p - 1) * ps).Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}
	return questions, total, nil
}

// ListQuestionsByTypeDifficulty returns all matching questions (no paging) for paper generation.
func (r *Repository) ListQuestionsByTypeDifficulty(ctx context.Context, qtype, difficulty string) ([]model.Question, error) {
	q := r.db.WithContext(ctx).Model(&model.Question{}).Where("type = ?", qtype)
	if difficulty != "" {
		q = q.Where("difficulty = ?", difficulty)
	}
	var questions []model.Question
	if err := q.Order("id ASC").Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list questions by type difficulty: %w", err)
	}
	return questions, nil
}

// FindQuestionsByIDs returns questions keyed by primary key.
func (r *Repository) FindQuestionsByIDs(ctx context.Context, ids []uint) (map[uint]model.Question, error) {
	result := make(map[uint]model.Question, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var questions []model.Question
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("find questions by ids: %w", err)
	}
	for _, q := range questions {
		result[q.ID] = q
	}
	return result, nil
}

// CountQuestions returns the total number of questions.
func (r *Repository) CountQuestions(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Question{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count questions: %w", err)
	}
	return total, nil
}
