package model

import "time"

// ExamAttempt is a single student session for an exam.
type ExamAttempt struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ExamID         uint       `gorm:"index;not null" json:"exam_id"`
	StudentID      uint       `gorm:"index;not null" json:"student_id"`
	Status         string     `gorm:"size:16;not null;default:in_progress" json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	Deadline       time.Time  `json:"deadline"`
	QuestionOrder  string     `gorm:"type:text" json:"-"`
	OptionOrder    string     `gorm:"type:text" json:"-"`
	ObjectiveScore float64    `gorm:"not null;default:0" json:"objective_score"`
	TotalScore     float64    `gorm:"not null;default:0" json:"total_score"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Answer is one student answer for one exam question.
type Answer struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	AttemptID      uint       `gorm:"uniqueIndex:idx_attempt_question;not null" json:"attempt_id"`
	ExamQuestionID uint       `gorm:"uniqueIndex:idx_attempt_question;not null" json:"exam_question_id"`
	QuestionID     uint       `gorm:"index;not null" json:"question_id"`
	AnswerText     string     `gorm:"type:text" json:"answer_text"`
	IsCorrect      *bool      `json:"is_correct"`
	Score          float64    `gorm:"not null;default:0" json:"score"`
	Marked         bool       `gorm:"not null;default:false" json:"marked"`
	GradedBy       uint       `json:"graded_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
