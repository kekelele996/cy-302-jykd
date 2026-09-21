package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

// CreateVersion inserts one paper version.
func (r *Repository) CreateVersion(ctx context.Context, v *model.ExamVersion) error {
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return fmt.Errorf("create exam version: %w", err)
	}
	return nil
}

// CreateVersionQuestions inserts frozen question snapshots in bulk.
func (r *Repository) CreateVersionQuestions(ctx context.Context, items []model.ExamVersionQuestion) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&items).Error; err != nil {
		return fmt.Errorf("create exam version questions: %w", err)
	}
	return nil
}

// ReplaceDraftVersionQuestions atomically swaps the question snapshots of a
// draft version. Published or archived versions are immutable.
func (r *Repository) ReplaceDraftVersionQuestions(ctx context.Context, versionID uint, items []model.ExamVersionQuestion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v model.ExamVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, versionID).Error; err != nil {
			return wrapQuery("find version for replace", err)
		}
		if v.Status != constants.VersionDraft {
			return fmt.Errorf("%w: 已发布版本不可修改，请重新组卷生成新版本", ErrConflict)
		}
		if err := tx.Where("version_id = ?", versionID).Delete(&model.ExamVersionQuestion{}).Error; err != nil {
			return fmt.Errorf("delete draft version questions: %w", err)
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("create draft version questions: %w", err)
			}
		}
		return nil
	})
}

// FindVersionByID returns one paper version.
func (r *Repository) FindVersionByID(ctx context.Context, id uint) (*model.ExamVersion, error) {
	var v model.ExamVersion
	if err := r.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, wrapQuery("find exam version by id", err)
	}
	return &v, nil
}

// FindVersionByExamAndNo returns one version of an exam by its version number.
func (r *Repository) FindVersionByExamAndNo(ctx context.Context, examID uint, versionNo int) (*model.ExamVersion, error) {
	var v model.ExamVersion
	err := r.db.WithContext(ctx).
		Where("exam_id = ? AND version_no = ?", examID, versionNo).
		First(&v).Error
	if err != nil {
		return nil, wrapQuery("find exam version by no", err)
	}
	return &v, nil
}

// FindDraftVersion returns the pending draft version of an exam, if any.
func (r *Repository) FindDraftVersion(ctx context.Context, examID uint) (*model.ExamVersion, error) {
	var v model.ExamVersion
	err := r.db.WithContext(ctx).
		Where("exam_id = ? AND status = ?", examID, constants.VersionDraft).
		Order("version_no DESC").First(&v).Error
	if err != nil {
		return nil, wrapQuery("find draft version", err)
	}
	return &v, nil
}

// FindCurrentVersion returns the version currently served to new attempts.
// exam.current_version_id is authoritative; the latest published version is a
// fallback for manually seeded data.
func (r *Repository) FindCurrentVersion(ctx context.Context, examID uint) (*model.ExamVersion, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).Select("id", "current_version_id").First(&exam, examID).Error; err != nil {
		return nil, wrapQuery("find exam for current version", err)
	}
	if exam.CurrentVersionID != 0 {
		var v model.ExamVersion
		if err := r.db.WithContext(ctx).First(&v, exam.CurrentVersionID).Error; err == nil {
			return &v, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, wrapQuery("find current version by pointer", err)
		}
	}
	var v model.ExamVersion
	err := r.db.WithContext(ctx).
		Where("exam_id = ? AND status = ?", examID, constants.VersionPublished).
		Order("version_no DESC").First(&v).Error
	if err != nil {
		return nil, wrapQuery("find current published version", err)
	}
	return &v, nil
}

// ListVersions returns all versions of an exam, newest first.
func (r *Repository) ListVersions(ctx context.Context, examID uint) ([]model.ExamVersion, error) {
	var versions []model.ExamVersion
	if err := r.db.WithContext(ctx).
		Where("exam_id = ?", examID).
		Order("version_no DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list exam versions: %w", err)
	}
	return versions, nil
}

// NextVersionNo returns max(version_no)+1 for an exam.
func (r *Repository) NextVersionNo(ctx context.Context, examID uint) (int, error) {
	return nextVersionNo(r.db.WithContext(ctx), examID)
}

// ListVersionQuestions returns frozen snapshots of one version ordered.
func (r *Repository) ListVersionQuestions(ctx context.Context, versionID uint) ([]model.ExamVersionQuestion, error) {
	var items []model.ExamVersionQuestion
	if err := r.db.WithContext(ctx).
		Where("version_id = ?", versionID).
		Order("sort_order ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list version questions: %w", err)
	}
	return items, nil
}

// CountVersionQuestions returns the number of frozen questions in a version.
func (r *Repository) CountVersionQuestions(ctx context.Context, versionID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.ExamVersionQuestion{}).
		Where("version_id = ?", versionID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count version questions: %w", err)
	}
	return count, nil
}

// DeleteVersions removes all versions and snapshots of an exam.
func (r *Repository) DeleteVersions(ctx context.Context, examID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&model.ExamVersion{}).Where("exam_id = ?", examID).Pluck("id", &ids).Error; err != nil {
			return fmt.Errorf("pluck version ids: %w", err)
		}
		if len(ids) > 0 {
			if err := tx.Where("version_id IN ?", ids).Delete(&model.ExamVersionQuestion{}).Error; err != nil {
				return fmt.Errorf("delete version questions: %w", err)
			}
		}
		if err := tx.Where("exam_id = ?", examID).Delete(&model.ExamVersion{}).Error; err != nil {
			return fmt.Errorf("delete exam versions: %w", err)
		}
		return nil
	})
}

// DraftComposer builds the snapshot rows and totals for a new draft version.
type DraftComposer func() (items []model.ExamVersionQuestion, total float64, duration int, err error)

// CreateExamWithDraftVersion inserts an exam and its first draft version with
// snapshots in a single transaction, eliminating any window where an exam
// exists without a paper.
func (r *Repository) CreateExamWithDraftVersion(ctx context.Context, exam *model.Exam, compose DraftComposer) (*model.ExamVersion, error) {
	var result *model.ExamVersion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(exam).Error; err != nil {
			return fmt.Errorf("create exam: %w", err)
		}
		items, total, duration, composeErr := compose()
		if composeErr != nil {
			return composeErr
		}
		version := &model.ExamVersion{
			ExamID:          exam.ID,
			VersionNo:       1,
			Status:          constants.VersionDraft,
			TotalScore:      total,
			DurationMinutes: duration,
			CreatedBy:       exam.CreatedBy,
		}
		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("create draft version: %w", err)
		}
		for i := range items {
			items[i].VersionID = version.ID
			items[i].ExamID = exam.ID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("create draft snapshots: %w", err)
			}
		}
		result = version
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SaveDraftVersion composes a fresh draft for an exam in one transaction.
//
//   - For a draft exam: the existing draft is replaced in place, so repeated
//     regeneration before publish keeps one draft version.
//   - For a published/closed exam: a new draft (new version number) is
//     appended; an existing pending draft is rejected so only one pending
//     change can exist.
//
// The exam row is locked for the whole composition, serializing concurrent
// regenerations and publishes ("only one takes effect").
func (r *Repository) SaveDraftVersion(ctx context.Context, examID uint, compose DraftComposer) (*model.ExamVersion, error) {
	var result *model.ExamVersion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exam, err := lockExam(tx, examID)
		if err != nil {
			return err
		}
		var draft *model.ExamVersion
		if existing, fErr := findDraftVersionTx(tx, examID); fErr == nil {
			draft = existing
		} else if !errors.Is(fErr, ErrNotFound) {
			return fErr
		}
		if exam.Status != constants.ExamDraft && draft != nil {
			return fmt.Errorf("%w: 已有待发布的新版本，请先发布或放弃该版本", ErrConflict)
		}

		items, total, duration, composeErr := compose()
		if composeErr != nil {
			return composeErr
		}

		if draft != nil {
			if err := tx.Where("version_id = ?", draft.ID).Delete(&model.ExamVersionQuestion{}).Error; err != nil {
				return fmt.Errorf("delete old draft snapshots: %w", err)
			}
			for i := range items {
				items[i].VersionID = draft.ID
				items[i].ExamID = examID
			}
			if len(items) > 0 {
				if err := tx.Create(&items).Error; err != nil {
					return fmt.Errorf("create draft snapshots: %w", err)
				}
			}
			if err := tx.Model(&model.ExamVersion{}).Where("id = ?", draft.ID).Updates(map[string]any{
				"total_score":      total,
				"duration_minutes": duration,
			}).Error; err != nil {
				return fmt.Errorf("update draft version: %w", err)
			}
			draft.TotalScore = total
			draft.DurationMinutes = duration
			result = draft
			return nil
		}

		versionNo, err := nextVersionNo(tx, examID)
		if err != nil {
			return err
		}
		version := &model.ExamVersion{
			ExamID:          examID,
			VersionNo:       versionNo,
			Status:          constants.VersionDraft,
			TotalScore:      total,
			DurationMinutes: duration,
			CreatedBy:       exam.CreatedBy,
		}
		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("create draft version: %w", err)
		}
		for i := range items {
			items[i].VersionID = version.ID
			items[i].ExamID = examID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("create draft snapshots: %w", err)
			}
		}
		result = version
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// PublishVersion freezes and publishes the draft version of an exam in one
// transaction. The exam row is locked; a concurrent publish or regeneration
// is serialized. A second publish once the draft is gone fails with
// ErrConflict ("repeated publish takes effect only once").
func (r *Repository) PublishVersion(ctx context.Context, examID uint) (*model.ExamVersion, error) {
	var published *model.ExamVersion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exam, err := lockExam(tx, examID)
		if err != nil {
			return err
		}
		draft, err := findDraftVersionTx(tx, examID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				// No pending composition: repeated publish takes no effect.
				if exam.Status == constants.ExamPublished || exam.Status == constants.ExamClosed {
					return fmt.Errorf("%w: 考试已发布，重复发布不会生效", ErrConflict)
				}
				return fmt.Errorf("%w: 请先组卷再发布", ErrValidation)
			}
			return err
		}
		count, err := countVersionQuestionsTx(tx, draft.ID)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("%w: 试卷没有题目，无法发布", ErrValidation)
		}

		// Archive the previously active version (normally absent here because
		// an exam is only published again after a new draft was composed).
		if err := tx.Model(&model.ExamVersion{}).
			Where("exam_id = ? AND status = ?", examID, constants.VersionPublished).
			Update("status", constants.VersionArchived).Error; err != nil {
			return fmt.Errorf("archive previous version: %w", err)
		}

		now := time.Now()
		if err := tx.Model(&model.ExamVersion{}).Where("id = ?", draft.ID).Updates(map[string]any{
			"status":       constants.VersionPublished,
			"published_at": now,
		}).Error; err != nil {
			return fmt.Errorf("publish version: %w", err)
		}
		if err := tx.Model(&model.Exam{}).Where("id = ?", examID).Updates(map[string]any{
			"status":             constants.ExamPublished,
			"current_version_id": draft.ID,
			"total_score":        draft.TotalScore,
			"duration_minutes":   draft.DurationMinutes,
		}).Error; err != nil {
			return fmt.Errorf("update exam on publish: %w", err)
		}
		draft.Status = constants.VersionPublished
		draft.PublishedAt = &now
		published = draft
		return nil
	})
	if err != nil {
		return nil, err
	}
	return published, nil
}

// DiscardDraftVersion removes a pending draft version of an exam.
func (r *Repository) DiscardDraftVersion(ctx context.Context, examID, versionID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockExam(tx, examID); err != nil {
			return err
		}
		res := tx.Where("id = ? AND exam_id = ? AND status = ?", versionID, examID, constants.VersionDraft).
			Delete(&model.ExamVersion{})
		if res.Error != nil {
			return fmt.Errorf("discard draft version: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := tx.Where("version_id = ?", versionID).Delete(&model.ExamVersionQuestion{}).Error; err != nil {
			return fmt.Errorf("discard draft snapshots: %w", err)
		}
		return nil
	})
}

func lockExam(tx *gorm.DB, examID uint) (*model.Exam, error) {
	var exam model.Exam
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&exam, examID).Error; err != nil {
		return nil, wrapQuery("lock exam", err)
	}
	return &exam, nil
}

func findDraftVersionTx(tx *gorm.DB, examID uint) (*model.ExamVersion, error) {
	var v model.ExamVersion
	err := tx.
		Where("exam_id = ? AND status = ?", examID, constants.VersionDraft).
		Order("version_no DESC").First(&v).Error
	if err != nil {
		return nil, wrapQuery("find draft version", err)
	}
	return &v, nil
}

func countVersionQuestionsTx(tx *gorm.DB, versionID uint) (int64, error) {
	var count int64
	if err := tx.Model(&model.ExamVersionQuestion{}).Where("version_id = ?", versionID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count version questions: %w", err)
	}
	return count, nil
}

func nextVersionNo(db *gorm.DB, examID uint) (int, error) {
	var maxNo int
	if err := db.Model(&model.ExamVersion{}).
		Where("exam_id = ?", examID).
		Select("COALESCE(MAX(version_no), 0)").
		Scan(&maxNo).Error; err != nil {
		return 0, fmt.Errorf("next version no: %w", err)
	}
	return maxNo + 1, nil
}
