package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// ExamService handles exam creation, paper generation and lifecycle.
type ExamService struct {
	baseService
	repo         ExamRepo
	questionRepo QuestionRepo
}

// NewExamService constructs ExamService.
func NewExamService(repo ExamRepo, questionRepo QuestionRepo, logger *slog.Logger) *ExamService {
	return &ExamService{baseService: NewBaseService(logger), repo: repo, questionRepo: questionRepo}
}

// Create builds an exam and auto-generates its paper.
func (s *ExamService) Create(ctx context.Context, createdBy uint, req dto.ExamCreateRequest) (*dto.ExamResponse, error) {
	var computedTotal float64
	var items []model.ExamQuestion
	order := 0
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, cfg := range req.QuestionConfig {
		questions, err := s.questionRepo.ListQuestionsByTypeDifficulty(ctx, cfg.Type, cfg.Difficulty)
		if err != nil {
			return nil, fmt.Errorf("list questions by type difficulty: %w", err)
		}
		if len(questions) < cfg.Count {
			return nil, fmt.Errorf("%w: 题型 %s 难度 %s 题库数量不足（需要 %d，实际 %d）", ErrValidation, cfg.Type, cfg.Difficulty, cfg.Count, len(questions))
		}
		shuffle(questions, rng)
		for i := 0; i < cfg.Count; i++ {
			items = append(items, model.ExamQuestion{
				ExamID:     0,
				QuestionID: questions[i].ID,
				Score:      cfg.Score,
				SortOrder:  order,
			})
			computedTotal += cfg.Score
			order++
		}
	}

	if req.TotalScore > 0 && req.TotalScore != computedTotal {
		return nil, fmt.Errorf("%w: 总分 %.2f 与各题型分值之和 %.2f 不一致", ErrValidation, req.TotalScore, computedTotal)
	}

	exam := &model.Exam{
		Title:           req.Title,
		Description:     req.Description,
		TotalScore:      computedTotal,
		DurationMinutes: req.DurationMinutes,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Status:          constants.ExamDraft,
		CreatedBy:       createdBy,
	}
	if err := s.repo.CreateExam(ctx, exam); err != nil {
		return nil, fmt.Errorf("create exam: %w", err)
	}
	for i := range items {
		items[i].ExamID = exam.ID
	}
	if err := s.repo.ReplaceExamQuestions(ctx, exam.ID, items); err != nil {
		return nil, fmt.Errorf("replace exam questions: %w", err)
	}
	return s.toResponse(ctx, exam)
}

// List returns exams based on the caller role.
func (s *ExamService) List(ctx context.Context, role string, userID uint, query dto.ExamListQuery) (dto.PageResult, error) {
	filter := repository.ExamFilter{Status: query.Status, Keyword: query.Keyword}
	switch role {
	case constants.RoleAdmin:
		// admin sees all exams
	case constants.RoleTeacher:
		filter.CreatedBy = userID
	case constants.RoleStudent:
		if query.Status == "" {
			filter.Status = constants.ExamPublished
		}
	default:
		return dto.PageResult{}, ErrForbidden
	}

	exams, total, err := s.repo.ListExams(ctx, filter, query.Page, query.PageSize)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("list exams: %w", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.ExamResponse, 0, len(exams))
	for i := range exams {
		resp, err := s.toResponse(ctx, &exams[i])
		if err != nil {
			return dto.PageResult{}, err
		}
		items = append(items, *resp)
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get returns one exam with access checks.
func (s *ExamService) Get(ctx context.Context, role string, userID, id uint) (*dto.ExamResponse, error) {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleStudent && exam.Status != constants.ExamPublished {
		return nil, ErrNotFound
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	return s.toResponse(ctx, exam)
}

// Publish makes a draft exam available to students.
func (s *ExamService) Publish(ctx context.Context, role string, userID, id uint) error {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return ErrForbidden
	}
	count, err := s.repo.CountExamQuestions(ctx, id)
	if err != nil {
		return fmt.Errorf("count exam questions: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("%w: 试卷没有题目，无法发布", ErrValidation)
	}
	exam.Status = constants.ExamPublished
	if err := s.repo.UpdateExam(ctx, exam); err != nil {
		return fmt.Errorf("publish exam: %w", err)
	}
	return nil
}

// Close stops new attempts for an exam.
func (s *ExamService) Close(ctx context.Context, role string, userID, id uint) error {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return ErrForbidden
	}
	exam.Status = constants.ExamClosed
	if err := s.repo.UpdateExam(ctx, exam); err != nil {
		return fmt.Errorf("close exam: %w", err)
	}
	return nil
}

// Delete removes an exam (only creator/admin).
func (s *ExamService) Delete(ctx context.Context, role string, userID, id uint) error {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return ErrForbidden
	}
	if err := s.repo.DeleteExam(ctx, id); err != nil {
		return fmt.Errorf("delete exam: %w", err)
	}
	return nil
}

// ListPaperQuestions returns the full paper with answers (teacher/admin only).
func (s *ExamService) ListPaperQuestions(ctx context.Context, role string, userID, id uint) ([]dto.ExamQuestionResponse, error) {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	if role == constants.RoleStudent {
		return nil, ErrForbidden
	}
	items, err := s.repo.ListExamQuestions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list exam questions: %w", err)
	}
	ids := make([]uint, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find questions by ids: %w", err)
	}
	result := make([]dto.ExamQuestionResponse, 0, len(items))
	for _, it := range items {
		q, ok := questionMap[it.QuestionID]
		if !ok {
			continue
		}
		result = append(result, dto.ExamQuestionResponse{ID: it.ID, Score: it.Score, Question: *questionToResponse(&q)})
	}
	return result, nil
}

// CountQuestions exposes question count for exam metadata.
func (s *ExamService) CountQuestions(ctx context.Context, id uint) (int64, error) {
	return s.repo.CountExamQuestions(ctx, id)
}

func (s *ExamService) toResponse(ctx context.Context, exam *model.Exam) (*dto.ExamResponse, error) {
	count, err := s.repo.CountExamQuestions(ctx, exam.ID)
	if err != nil {
		return nil, fmt.Errorf("count exam questions: %w", err)
	}
	return &dto.ExamResponse{
		ID:              exam.ID,
		Title:           exam.Title,
		Description:     exam.Description,
		TotalScore:      exam.TotalScore,
		DurationMinutes: exam.DurationMinutes,
		StartTime:       exam.StartTime,
		EndTime:         exam.EndTime,
		Status:          exam.Status,
		QuestionCount:   int(count),
		CreatedBy:       exam.CreatedBy,
		CreatedAt:       exam.CreatedAt,
	}, nil
}
