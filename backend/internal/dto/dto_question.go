package dto

import "time"

// Option is one choice option for a question.
type Option struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// QuestionRequest creates or updates a question.
type QuestionRequest struct {
	Type           string   `json:"type" binding:"required,oneof=single multiple true_false fill_blank short_answer"`
	Content        string   `json:"content" binding:"required"`
	Options        []Option `json:"options"`
	Answer         any      `json:"answer" binding:"required"`
	Analysis       string   `json:"analysis"`
	Difficulty     string   `json:"difficulty" binding:"required,oneof=easy medium hard"`
	KnowledgePoint string   `json:"knowledge_point" binding:"required,max=128"`
	Score          float64  `json:"score" binding:"required,min=0.5"`
}

// QuestionListQuery filters the question bank.
type QuestionListQuery struct {
	PageQuery
	Type           string `form:"type" binding:"omitempty,oneof=single multiple true_false fill_blank short_answer"`
	Difficulty     string `form:"difficulty" binding:"omitempty,oneof=easy medium hard"`
	KnowledgePoint string `form:"knowledge_point"`
	Keyword        string `form:"keyword"`
}

// QuestionResponse is returned for teacher/admin facing question reads.
type QuestionResponse struct {
	ID             uint      `json:"id"`
	Type           string    `json:"type"`
	Content        string    `json:"content"`
	Options        []Option  `json:"options"`
	Answer         any       `json:"answer,omitempty"`
	Analysis       string    `json:"analysis"`
	Difficulty     string    `json:"difficulty"`
	KnowledgePoint string    `json:"knowledge_point"`
	Score          float64   `json:"score"`
	CreatedBy      uint      `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// BatchImportResult reports how many questions were imported.
type BatchImportResult struct {
	Imported int      `json:"imported"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors"`
}
