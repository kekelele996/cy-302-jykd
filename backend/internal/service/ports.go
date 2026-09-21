package service

import (
	"context"

	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// UserRepo is the user persistence contract.
type UserRepo interface {
	CreateUser(ctx context.Context, u *model.User) error
	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	ListUsers(ctx context.Context, keyword, role string, page, pageSize int) ([]model.User, int64, error)
	UpdateUserStatus(ctx context.Context, id uint, status string) error
}

// QuestionRepo is the question persistence contract.
type QuestionRepo interface {
	CreateQuestion(ctx context.Context, q *model.Question) error
	CreateQuestionsBatch(ctx context.Context, questions []model.Question) error
	FindQuestionByID(ctx context.Context, id uint) (*model.Question, error)
	UpdateQuestion(ctx context.Context, q *model.Question) error
	DeleteQuestion(ctx context.Context, id uint) error
	ListQuestions(ctx context.Context, filter repository.QuestionFilter, page, pageSize int) ([]model.Question, int64, error)
	ListQuestionsByTypeDifficulty(ctx context.Context, qtype, difficulty string) ([]model.Question, error)
	FindQuestionsByIDs(ctx context.Context, ids []uint) (map[uint]model.Question, error)
}

// ExamRepo is the exam persistence contract.
type ExamRepo interface {
	CreateExam(ctx context.Context, exam *model.Exam) error
	FindExamByID(ctx context.Context, id uint) (*model.Exam, error)
	UpdateExam(ctx context.Context, exam *model.Exam) error
	DeleteExam(ctx context.Context, id uint) error
	ListExams(ctx context.Context, filter repository.ExamFilter, page, pageSize int) ([]model.Exam, int64, error)
	ReplaceExamQuestions(ctx context.Context, examID uint, items []model.ExamQuestion) error
	ListExamQuestions(ctx context.Context, examID uint) ([]model.ExamQuestion, error)
	CountExamQuestions(ctx context.Context, examID uint) (int64, error)
}

// AttemptRepo is the attempt persistence contract.
type AttemptRepo interface {
	CreateAttempt(ctx context.Context, a *model.ExamAttempt) error
	FindAttemptByID(ctx context.Context, id uint) (*model.ExamAttempt, error)
	UpdateAttempt(ctx context.Context, a *model.ExamAttempt) error
	FindInProgressAttempt(ctx context.Context, examID, studentID uint) (*model.ExamAttempt, error)
	ListAttemptsByStudent(ctx context.Context, studentID, examID uint, page, pageSize int) ([]model.ExamAttempt, int64, error)
	ListAttemptsByExam(ctx context.Context, examID uint) ([]model.ExamAttempt, error)
}

// AnswerRepo is the answer persistence contract.
type AnswerRepo interface {
	SaveAnswer(ctx context.Context, answer *model.Answer) error
	ListAnswersByAttempt(ctx context.Context, attemptID uint) ([]model.Answer, error)
}

// WrongRepo is the wrong-question persistence contract.
type WrongRepo interface {
	UpsertWrongQuestion(ctx context.Context, w *model.WrongQuestion) error
	ListWrongQuestions(ctx context.Context, studentID uint, knowledgePoint string, page, pageSize int) ([]model.WrongQuestion, int64, error)
	DeleteWrongQuestion(ctx context.Context, id, studentID uint) error
	MarkWrongQuestionResolved(ctx context.Context, id, studentID uint) error
}

// StatsRepo is the minimal persistence contract used by statistics queries.
type StatsRepo interface {
	FindExamByID(ctx context.Context, id uint) (*model.Exam, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	ListAttemptsByExam(ctx context.Context, examID uint) ([]model.ExamAttempt, error)
	CountUsers(ctx context.Context) (int64, error)
	CountQuestions(ctx context.Context) (int64, error)
	CountExams(ctx context.Context) (int64, error)
	CountAttempts(ctx context.Context) (int64, error)
}
