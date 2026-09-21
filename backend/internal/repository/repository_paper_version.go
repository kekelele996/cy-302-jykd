package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/model"
)

// FreezePaperInput carries the data needed to build one immutable paper version.
type FreezePaperInput struct {
	ExamID    uint
	VersionNo int
	Title     string
	CreatedBy uint
	Items     []FreezePaperItem
}

// FreezePaperItem is one question copied into the frozen version.
type FreezePaperItem struct {
	QuestionID     uint
	Type           string
	Content        string
	Options        string
	Answer         string
	Analysis       string
	Difficulty     string
	KnowledgePoint string
	Score          float64
	SortOrder      int
}

// WithTransaction runs fn inside a database transaction.
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

// CreatePaperVersion stores a new immutable version with its question snapshots
// inside the provided transaction. The caller is responsible for computing the
// next VersionNo while holding the exam row lock.
func (r *Repository) CreatePaperVersion(ctx context.Context, tx *gorm.DB, in FreezePaperInput) (*model.PaperVersion, error) {
	totalScore := 0.0
	for _, it := range in.Items {
		totalScore += it.Score
	}
	version := &model.PaperVersion{
		ExamID:     in.ExamID,
		VersionNo:  in.VersionNo,
		Title:      in.Title,
		TotalScore: totalScore,
		SnapshotAt: time.Now(),
		CreatedBy:  in.CreatedBy,
	}
	if err := tx.WithContext(ctx).Create(version).Error; err != nil {
		return nil, fmt.Errorf("create paper version: %w", err)
	}
	items := make([]model.PaperVersionQuestion, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, model.PaperVersionQuestion{
			VersionID:      version.ID,
			QuestionID:     it.QuestionID,
			Type:           it.Type,
			Content:        it.Content,
			Options:        it.Options,
			Answer:         it.Answer,
			Analysis:       it.Analysis,
			Difficulty:     it.Difficulty,
			KnowledgePoint: it.KnowledgePoint,
			Score:          it.Score,
			SortOrder:      it.SortOrder,
		})
	}
	if len(items) > 0 {
		if err := tx.WithContext(ctx).Create(&items).Error; err != nil {
			return nil, fmt.Errorf("create paper version questions: %w", err)
		}
	}
	return version, nil
}

// FindPaperVersionByID returns one frozen version.
func (r *Repository) FindPaperVersionByID(ctx context.Context, id uint) (*model.PaperVersion, error) {
	var version model.PaperVersion
	err := r.db.WithContext(ctx).First(&version, id).Error
	if err != nil {
		return nil, wrapQuery("find paper version by id", err)
	}
	return &version, nil
}

// FindCurrentPaperVersion returns the version an exam currently points at.
func (r *Repository) FindCurrentPaperVersion(ctx context.Context, examID uint) (*model.PaperVersion, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).Select("id", "current_version_id").First(&exam, examID).Error; err != nil {
		return nil, wrapQuery("find exam for current version", err)
	}
	if exam.CurrentVersionID == 0 {
		return nil, ErrNotFound
	}
	return r.FindPaperVersionByID(ctx, exam.CurrentVersionID)
}

// ListPaperVersions returns all frozen versions of an exam, newest first.
func (r *Repository) ListPaperVersions(ctx context.Context, examID uint) ([]model.PaperVersion, error) {
	var versions []model.PaperVersion
	if err := r.db.WithContext(ctx).
		Where("exam_id = ?", examID).
		Order("version_no DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list paper versions: %w", err)
	}
	return versions, nil
}

// MaxPaperVersionNo returns the highest version number for an exam (0 if none).
func (r *Repository) MaxPaperVersionNo(ctx context.Context, tx *gorm.DB, examID uint) (int, error) {
	var maxNo int
	if err := tx.WithContext(ctx).Model(&model.PaperVersion{}).
		Where("exam_id = ?", examID).
		Select("COALESCE(MAX(version_no), 0)").
		Scan(&maxNo).Error; err != nil {
		return 0, fmt.Errorf("max paper version no: %w", err)
	}
	return maxNo, nil
}

// ListPaperVersionQuestions returns the frozen questions of one version.
func (r *Repository) ListPaperVersionQuestions(ctx context.Context, versionID uint) ([]model.PaperVersionQuestion, error) {
	var items []model.PaperVersionQuestion
	if err := r.db.WithContext(ctx).
		Where("version_id = ?", versionID).
		Order("sort_order ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list paper version questions: %w", err)
	}
	return items, nil
}

// LockExamForUpdate loads an exam with a row lock; must run inside a transaction.
func (r *Repository) LockExamForUpdate(ctx context.Context, tx *gorm.DB, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := tx.WithContext(ctx).Clauses(clauseForUpdate()).First(&exam, id).Error; err != nil {
		return nil, wrapQuery("lock exam for update", err)
	}
	return &exam, nil
}

// PointExamToVersion atomically sets the current version pointer and status,
// and bumps the revision counter.
func (r *Repository) PointExamToVersion(ctx context.Context, tx *gorm.DB, examID, versionID uint, status string) error {
	res := tx.WithContext(ctx).Model(&model.Exam{}).
		Where("id = ?", examID).
		Updates(map[string]any{
			"current_version_id": versionID,
			"status":             status,
			"revision":           gorm.Expr("revision + 1"),
		})
	if res.Error != nil {
		return fmt.Errorf("point exam to version: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
