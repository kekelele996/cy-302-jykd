package model

import "time"

// ExamVersion is an immutable snapshot of a paper generated at one point in time.
//
// A version starts its life as "draft" right after auto paper generation. The
// first publish promotes that draft to "published". When a teacher regenerates
// a paper for an exam that already has a published version, a brand new draft
// version is created; publishing it archives the previously published version
// ("archived"). Snapshots are never mutated, so in-progress attempts and
// historical results always render the exact paper used at composition time.
type ExamVersion struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ExamID          uint       `gorm:"uniqueIndex:uk_exam_version_no;not null" json:"exam_id"`
	VersionNo       int        `gorm:"uniqueIndex:uk_exam_version_no;not null" json:"version_no"`
	Status          string     `gorm:"size:16;not null;default:draft;index:idx_version_exam_status,priority:2" json:"status"`
	TotalScore      float64    `gorm:"not null;default:0" json:"total_score"`
	DurationMinutes int        `gorm:"not null;default:60" json:"duration_minutes"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedBy       uint       `gorm:"index" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ExamVersionQuestion is a frozen question snapshot belonging to one version.
// It copies the question bank content at composition time so later edits or
// withdrawal (soft delete) of the source question never affect the paper.
type ExamVersionQuestion struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	VersionID      uint    `gorm:"uniqueIndex:uk_version_sort;index;not null" json:"version_id"`
	ExamID         uint    `gorm:"index;not null" json:"exam_id"`
	QuestionID     uint    `gorm:"index;not null" json:"question_id"`
	SortOrder      int     `gorm:"uniqueIndex:uk_version_sort;not null" json:"sort_order"`
	Score          float64 `gorm:"not null" json:"score"`
	Type           string  `gorm:"size:16;not null" json:"type"`
	Content        string  `gorm:"type:text;not null" json:"content"`
	Options        string  `gorm:"type:text" json:"options"`
	Answer         string  `gorm:"type:text;not null" json:"answer"`
	Analysis       string  `gorm:"type:text" json:"analysis"`
	Difficulty     string  `gorm:"size:16;not null" json:"difficulty"`
	KnowledgePoint string  `gorm:"size:128;not null" json:"knowledge_point"`
}
