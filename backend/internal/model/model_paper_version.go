package model

import "time"

// PaperVersion is an immutable snapshot of one exam paper taken at publish time.
// Re-publishing (re-generating the paper) always creates a new row; old versions
// are retained so published exams, in-progress attempts and historical grades
// keep showing exactly the content that was frozen when the version was created.
type PaperVersion struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ExamID     uint      `gorm:"uniqueIndex:uk_paper_version_no;not null" json:"exam_id"`
	VersionNo  int       `gorm:"uniqueIndex:uk_paper_version_no;not null" json:"version_no"`
	Title      string    `gorm:"size:128;not null" json:"title"`
	TotalScore float64   `gorm:"not null;default:0" json:"total_score"`
	SnapshotAt time.Time `gorm:"not null" json:"snapshot_at"`
	CreatedBy  uint      `gorm:"index" json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// PaperVersionQuestion is one frozen question inside a PaperVersion.
// The question stem, options, reference answer, analysis and per-question score
// are copied verbatim from the question bank at snapshot time, so later edits or
// deletion of the source question never mutate already published papers.
type PaperVersionQuestion struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	VersionID      uint    `gorm:"uniqueIndex:uk_version_question_sort;index;not null" json:"version_id"`
	QuestionID     uint    `gorm:"index;not null" json:"question_id"`
	Type           string  `gorm:"size:16;not null" json:"type"`
	Content        string  `gorm:"type:text;not null" json:"content"`
	Options        string  `gorm:"type:text" json:"options"`
	Answer         string  `gorm:"type:text;not null" json:"answer"`
	Analysis       string  `gorm:"type:text" json:"analysis"`
	Difficulty     string  `gorm:"size:16;not null" json:"difficulty"`
	KnowledgePoint string  `gorm:"size:128;not null" json:"knowledge_point"`
	Score          float64 `gorm:"not null" json:"score"`
	SortOrder      int     `gorm:"uniqueIndex:uk_version_question_sort;not null" json:"sort_order"`
}
