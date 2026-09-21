package dto

import "time"

// AttemptSummary is used in student history lists.
type AttemptSummary struct {
	AttemptID      uint       `json:"attempt_id"`
	ExamID         uint       `json:"exam_id"`
	ExamTitle      string     `json:"exam_title"`
	Status         string     `json:"status"`
	ObjectiveScore float64    `json:"objective_score"`
	TotalScore     float64    `json:"total_score"`
	StartedAt      time.Time  `json:"started_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
}

// AttemptQuestionDetail is one question in an attempt review.
type AttemptQuestionDetail struct {
	ExamQuestionID uint      `json:"exam_question_id"`
	Type           string    `json:"type"`
	Content        string    `json:"content"`
	Options        []Option  `json:"options"`
	StudentAnswer  any       `json:"student_answer"`
	CorrectAnswer  any       `json:"correct_answer"`
	IsCorrect      *bool     `json:"is_correct"`
	Score          float64   `json:"score"`
	MaxScore       float64   `json:"max_score"`
	Analysis       string    `json:"analysis"`
	Marked         bool      `json:"marked"`
	Graded         bool      `json:"graded"`
}

// AttemptDetail is the full review of one attempt.
type AttemptDetail struct {
	AttemptID      uint                     `json:"attempt_id"`
	ExamID         uint                     `json:"exam_id"`
	ExamTitle      string                   `json:"exam_title"`
	Status         string                   `json:"status"`
	ObjectiveScore float64                  `json:"objective_score"`
	TotalScore     float64                  `json:"total_score"`
	StartedAt      time.Time                `json:"started_at"`
	SubmittedAt    *time.Time               `json:"submitted_at"`
	Deadline       time.Time                `json:"deadline"`
	Questions      []AttemptQuestionDetail  `json:"questions"`
}

// TypeScore breaks down score by question type.
type TypeScore struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Max   float64 `json:"max"`
	Count int     `json:"count"`
}

// ReportResponse is the score analysis shown after grading.
type ReportResponse struct {
	AttemptID      uint        `json:"attempt_id"`
	ExamID         uint        `json:"exam_id"`
	ExamTitle      string      `json:"exam_title"`
	TotalScore     float64     `json:"total_score"`
	ObjectiveScore float64     `json:"objective_score"`
	SubjectiveScore float64    `json:"subjective_score"`
	Accuracy       float64     `json:"accuracy"`
	Rank           int         `json:"rank"`
	Participants   int         `json:"participants"`
	TypeBreakdown  []TypeScore `json:"type_breakdown"`
	SubmittedAt    *time.Time  `json:"submitted_at"`
}

// GradeListResponse lists attempts waiting for subjective grading.
type GradeListResponse struct {
	Attempts []AttemptSummary `json:"attempts"`
	ExamID   uint             `json:"exam_id"`
	ExamTitle string          `json:"exam_title"`
}
