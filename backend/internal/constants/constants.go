package constants

// Roles
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleStudent = "student"
)

// Question types
const (
	QuestionSingle     = "single"
	QuestionMultiple   = "multiple"
	QuestionTrueFalse  = "true_false"
	QuestionFillBlank  = "fill_blank"
	QuestionShortAnswer = "short_answer"
)

// Difficulties
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// Exam statuses
const (
	ExamDraft     = "draft"
	ExamPublished = "published"
	ExamClosed    = "closed"
)

// Exam version statuses. A version is frozen while draft composition is
// editable (via replacement, never mutation), immutable once published, and
// archived when a newer version is published.
const (
	VersionDraft    = "draft"
	VersionPublished = "published"
	VersionArchived  = "archived"
)

// Attempt statuses
const (
	AttemptInProgress = "in_progress"
	AttemptSubmitted  = "submitted"
)

// Wrong question statuses
const (
	WrongUnresolved = "unresolved"
	WrongResolved   = "resolved"
)

// User statuses
const (
	UserActive   = "active"
	UserDisabled = "disabled"
)
