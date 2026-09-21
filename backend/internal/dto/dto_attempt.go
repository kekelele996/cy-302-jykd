package dto

import "time"

// ExamQuestionView is sent to a student while taking an exam (no answer).
type ExamQuestionView struct {
	ExamQuestionID uint     `json:"exam_question_id"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	Options        []Option `json:"options"`
	Score          float64  `json:"score"`
	Marked         bool     `json:"marked"`
	Answer         any      `json:"answer,omitempty"`
}

// AttemptStartResponse returns the shuffled paper for a student.
type AttemptStartResponse struct {
	AttemptID        uint               `json:"attempt_id"`
	ExamID           uint               `json:"exam_id"`
	Title            string             `json:"title"`
	DurationMinutes  int                `json:"duration_minutes"`
	TotalScore       float64            `json:"total_score"`
	StartedAt        time.Time          `json:"started_at"`
	Deadline         time.Time          `json:"deadline"`
	Questions        []ExamQuestionView `json:"questions"`
}

// AnswerSubmitRequest saves one answer (optionally marks it).
type AnswerSubmitRequest struct {
	ExamQuestionID uint  `json:"exam_question_id" binding:"required"`
	Answer         any   `json:"answer"`
	Marked         *bool `json:"marked"`
}

// GradeItemRequest grades one subjective answer.
type GradeItemRequest struct {
	ExamQuestionID uint    `json:"exam_question_id" binding:"required"`
	Score          float64 `json:"score" binding:"min=0"`
}

// GradeRequest grades one or more subjective answers.
type GradeRequest struct {
	Items []GradeItemRequest `json:"items" binding:"required,min=1,dive"`
}

// AttemptListQuery filters attempt list for students.
type AttemptListQuery struct {
	PageQuery
	ExamID uint `form:"exam_id"`
}
