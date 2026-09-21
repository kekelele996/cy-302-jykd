package model

import "time"

// WrongQuestion records a student's incorrectly answered question.
type WrongQuestion struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	StudentID      uint      `gorm:"index;not null" json:"student_id"`
	QuestionID     uint      `gorm:"index;not null" json:"question_id"`
	KnowledgePoint string    `gorm:"size:128;index" json:"knowledge_point"`
	WrongCount     int       `gorm:"not null;default:1" json:"wrong_count"`
	LastWrongAt    time.Time `json:"last_wrong_at"`
	Status         string    `gorm:"size:16;not null;default:unresolved" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
