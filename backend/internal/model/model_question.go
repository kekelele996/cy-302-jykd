package model

import (
	"time"

	"gorm.io/gorm"
)

// Question stores one question in the question bank. Questions are soft
// deleted ("withdrawn"); frozen paper snapshots keep their own copies, so
// withdrawal never changes published, in-progress or historical exams.
type Question struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Type           string         `gorm:"size:16;not null;index" json:"type"`
	Content        string         `gorm:"type:text;not null" json:"content"`
	Options        string         `gorm:"type:text" json:"options"`
	Answer         string         `gorm:"type:text;not null" json:"answer"`
	Analysis       string         `gorm:"type:text" json:"analysis"`
	Difficulty     string         `gorm:"size:16;not null;index" json:"difficulty"`
	KnowledgePoint string         `gorm:"size:128;not null;index" json:"knowledge_point"`
	Score          float64        `gorm:"not null;default:1" json:"score"`
	CreatedBy      uint           `gorm:"index" json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
