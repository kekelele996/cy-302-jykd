package model

import "time"

// Exam represents a paper template created by a teacher/admin.
type Exam struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Title           string     `gorm:"size:128;not null" json:"title"`
	Description     string     `gorm:"type:text" json:"description"`
	TotalScore      float64    `gorm:"not null;default:0" json:"total_score"`
	DurationMinutes int        `gorm:"not null;default:60" json:"duration_minutes"`
	StartTime       *time.Time `json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	Status          string     `gorm:"size:16;not null;default:draft;index" json:"status"`
	CurrentVersionID uint      `gorm:"not null;default:0" json:"current_version_id"`
	CreatedBy       uint       `gorm:"index" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ExamQuestion is the legacy paper composition table used by schema 0001.
// It is retained only for the one-time backfill into ExamVersion snapshots
// (migration 0002) and is no longer read or written by business code.
type ExamQuestion struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	ExamID     uint    `gorm:"index;not null" json:"exam_id"`
	QuestionID uint    `gorm:"index;not null" json:"question_id"`
	Score      float64 `gorm:"not null" json:"score"`
	SortOrder  int     `gorm:"not null" json:"sort_order"`
}
