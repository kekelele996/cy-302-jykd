package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
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
	versionRepo  VersionRepo
	questionRepo QuestionRepo
}

// NewExamService constructs ExamService.
func NewExamService(repo ExamRepo, versionRepo VersionRepo, questionRepo QuestionRepo, logger *slog.Logger) *ExamService {
	return &ExamService{
		baseService:  NewBaseService(logger),
		repo:         repo,
		versionRepo:  versionRepo,
		questionRepo: questionRepo,
	}
}

// Create builds an exam and auto-generates its first draft paper version.
// The question content is snapshotted immediately, so later question-bank
// edits or withdrawal cannot alter this paper.
func (s *ExamService) Create(ctx context.Context, createdBy uint, req dto.ExamCreateRequest) (*dto.ExamResponse, error) {
	drawn, computedTotal, err := s.drawQuestions(ctx, req.QuestionConfig)
	if err != nil {
		return nil, err
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
	if _, err := s.versionRepo.CreateExamWithDraftVersion(ctx, exam, func() ([]model.ExamVersionQuestion, float64, int, error) {
		return toSnapshots(0, drawn, req.DurationMinutes), computedTotal, req.DurationMinutes, nil
	}); err != nil {
		return nil, fmt.Errorf("create exam with draft version: %w", err)
	}

	return s.toResponse(ctx, exam)
}

// Regenerate composes a paper again. A draft exam replaces its draft in
// place; a published/closed exam gets a new draft version while the published
// one stays frozen. Only one pending draft may exist.
func (s *ExamService) Regenerate(ctx context.Context, role string, userID, id uint, req dto.PaperRegenerateRequest) (*dto.ExamVersionResponse, error) {
	exam, err := s.loadOwnedExam(ctx, role, userID, id)
	if err != nil {
		return nil, err
	}
	if exam.Status == constants.ExamClosed {
		return nil, fmt.Errorf("%w: 考试已关闭，不能重新组卷", ErrValidation)
	}
	drawn, computedTotal, err := s.drawQuestions(ctx, req.QuestionConfig)
	if err != nil {
		return nil, err
	}
	if req.TotalScore > 0 && req.TotalScore != computedTotal {
		return nil, fmt.Errorf("%w: 总分 %.2f 与各题型分值之和 %.2f 不一致", ErrValidation, req.TotalScore, computedTotal)
	}

	duration := req.DurationMinutes
	if duration <= 0 {
		duration = exam.DurationMinutes
	}
	version, err := s.versionRepo.SaveDraftVersion(ctx, id, func() ([]model.ExamVersionQuestion, float64, int, error) {
		return toSnapshots(id, drawn, duration), computedTotal, duration, nil
	})
	if err != nil {
		return nil, err
	}

	// Draft exam keeps one mutable draft version; its metadata follows the
	// newest composition immediately.
	if exam.Status == constants.ExamDraft {
		exam.TotalScore = computedTotal
		exam.DurationMinutes = duration
		if strings.TrimSpace(req.Title) != "" {
			exam.Title = req.Title
		}
		exam.Description = req.Description
		if err := s.repo.UpdateExam(ctx, exam); err != nil {
			return nil, fmt.Errorf("update exam metadata: %w", err)
		}
	}
	return s.versionResponse(ctx, version), nil
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

// Publish makes the draft version available to students. Concurrent or
// repeated publishes are serialized by a row lock and only the first wins.
func (s *ExamService) Publish(ctx context.Context, role string, userID, id uint) error {
	if _, err := s.loadOwnedExam(ctx, role, userID, id); err != nil {
		return err
	}
	if _, err := s.versionRepo.PublishVersion(ctx, id); err != nil {
		return err
	}
	return nil
}

// Close stops new attempts for an exam; frozen versions are untouched.
func (s *ExamService) Close(ctx context.Context, role string, userID, id uint) error {
	exam, err := s.loadOwnedExam(ctx, role, userID, id)
	if err != nil {
		return err
	}
	exam.Status = constants.ExamClosed
	if err := s.repo.UpdateExam(ctx, exam); err != nil {
		return fmt.Errorf("close exam: %w", err)
	}
	return nil
}

// Delete removes an exam (only creator/admin). Frozen versions are removed;
// historical attempts still render their paper via the pinned version
// snapshots retained on the attempt side.
func (s *ExamService) Delete(ctx context.Context, role string, userID, id uint) error {
	if _, err := s.loadOwnedExam(ctx, role, userID, id); err != nil {
		return err
	}
	if err := s.repo.DeleteExam(ctx, id); err != nil {
		return fmt.Errorf("delete exam: %w", err)
	}
	return nil
}

// ListVersions lists every paper version of an exam (teachers/admins).
func (s *ExamService) ListVersions(ctx context.Context, role string, userID, id uint) ([]dto.ExamVersionResponse, error) {
	if _, err := s.loadOwnedExam(ctx, role, userID, id); err != nil {
		return nil, err
	}
	versions, err := s.versionRepo.ListVersions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	result := make([]dto.ExamVersionResponse, 0, len(versions))
	for i := range versions {
		result = append(result, *s.versionResponse(ctx, &versions[i]))
	}
	return result, nil
}

// GetVersion returns one version metadata.
func (s *ExamService) GetVersion(ctx context.Context, role string, userID, examID uint, versionNo int) (*dto.ExamVersionResponse, error) {
	if _, err := s.loadOwnedExam(ctx, role, userID, examID); err != nil {
		return nil, err
	}
	version, err := s.versionRepo.FindVersionByExamAndNo(ctx, examID, versionNo)
	if err != nil {
		return nil, err
	}
	return s.versionResponse(ctx, version), nil
}

// ListVersionQuestions returns the frozen paper of one version with answers.
func (s *ExamService) ListVersionQuestions(ctx context.Context, role string, userID, examID uint, versionNo int) ([]dto.ExamQuestionResponse, error) {
	if _, err := s.loadOwnedExam(ctx, role, userID, examID); err != nil {
		return nil, err
	}
	version, err := s.resolveVersion(ctx, examID, versionNo)
	if err != nil {
		return nil, err
	}
	return s.paperResponses(ctx, version.ID)
}

// ListPaperQuestions returns the active (or draft, if unpublished) paper for
// the legacy questions endpoint.
func (s *ExamService) ListPaperQuestions(ctx context.Context, role string, userID, id uint) ([]dto.ExamQuestionResponse, error) {
	exam, err := s.loadOwnedExam(ctx, role, userID, id)
	if err != nil {
		return nil, err
	}
	version, err := s.activeVersionForStaff(ctx, exam)
	if err != nil {
		return nil, err
	}
	return s.paperResponses(ctx, version.ID)
}

// DiscardDraft removes a pending (unpublished) new version.
func (s *ExamService) DiscardDraft(ctx context.Context, role string, userID, examID, versionID uint) error {
	if _, err := s.loadOwnedExam(ctx, role, userID, examID); err != nil {
		return err
	}
	return s.versionRepo.DiscardDraftVersion(ctx, examID, versionID)
}

// CountQuestions exposes question count for exam metadata.
func (s *ExamService) CountQuestions(ctx context.Context, id uint) (int64, error) {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return 0, err
	}
	version, err := s.activeVersionForStaff(ctx, exam)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return s.versionRepo.CountVersionQuestions(ctx, version.ID)
}

func (s *ExamService) loadOwnedExam(ctx context.Context, role string, userID, id uint) (*model.Exam, error) {
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
	return exam, nil
}

// activeVersionForStaff returns the draft for an unpublished exam, otherwise
// the currently published version.
func (s *ExamService) activeVersionForStaff(ctx context.Context, exam *model.Exam) (*model.ExamVersion, error) {
	if exam.Status == constants.ExamDraft {
		if v, err := s.versionRepo.FindDraftVersion(ctx, exam.ID); err == nil {
			return v, nil
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return s.versionRepo.FindCurrentVersion(ctx, exam.ID)
}

// resolveVersion maps versionNo<=0 to the active/current version.
func (s *ExamService) resolveVersion(ctx context.Context, examID uint, versionNo int) (*model.ExamVersion, error) {
	if versionNo <= 0 {
		return s.versionRepo.FindCurrentVersion(ctx, examID)
	}
	return s.versionRepo.FindVersionByExamAndNo(ctx, examID, versionNo)
}

func (s *ExamService) paperResponses(ctx context.Context, versionID uint) ([]dto.ExamQuestionResponse, error) {
	snapshots, err := s.versionRepo.ListVersionQuestions(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("list version questions: %w", err)
	}
	result := make([]dto.ExamQuestionResponse, 0, len(snapshots))
	for i := range snapshots {
		snap := snapshots[i]
		result = append(result, dto.ExamQuestionResponse{
			ID:       snap.ID,
			Score:    snap.Score,
			Question: snapshotToQuestionResponse(&snap),
		})
	}
	return result, nil
}

// drawnQuestion carries a picked question and the per-item score.
type drawnQuestion struct {
	question model.Question
	score    float64
}

func (s *ExamService) drawQuestions(ctx context.Context, configs []dto.PaperQuestionConfig) ([]drawnQuestion, float64, error) {
	var drawn []drawnQuestion
	var computedTotal float64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, cfg := range configs {
		questions, err := s.questionRepo.ListQuestionsByTypeDifficulty(ctx, cfg.Type, cfg.Difficulty)
		if err != nil {
			return nil, 0, fmt.Errorf("list questions by type difficulty: %w", err)
		}
		if len(questions) < cfg.Count {
			return nil, 0, fmt.Errorf("%w: 题型 %s 难度 %s 题库数量不足（需要 %d，实际 %d）", ErrValidation, cfg.Type, cfg.Difficulty, cfg.Count, len(questions))
		}
		shuffle(questions, rng)
		for i := 0; i < cfg.Count; i++ {
			drawn = append(drawn, drawnQuestion{question: questions[i], score: cfg.Score})
			computedTotal += cfg.Score
		}
	}
	return drawn, computedTotal, nil
}

func toSnapshots(examID uint, drawn []drawnQuestion, _ int) []model.ExamVersionQuestion {
	items := make([]model.ExamVersionQuestion, 0, len(drawn))
	for i, d := range drawn {
		q := d.question
		items = append(items, model.ExamVersionQuestion{
			ExamID:         examID,
			QuestionID:     q.ID,
			SortOrder:      i,
			Score:          d.score,
			Type:           q.Type,
			Content:        q.Content,
			Options:        q.Options,
			Answer:         q.Answer,
			Analysis:       q.Analysis,
			Difficulty:     q.Difficulty,
			KnowledgePoint: q.KnowledgePoint,
		})
	}
	return items
}

func snapshotToQuestionResponse(snap *model.ExamVersionQuestion) dto.QuestionResponse {
	options, _ := unmarshalOptions(snap.Options)
	answer, _ := unmarshalAnswer(snap.Answer)
	return dto.QuestionResponse{
		ID:             snap.QuestionID,
		Type:           snap.Type,
		Content:        snap.Content,
		Options:        options,
		Answer:         answer,
		Analysis:       snap.Analysis,
		Difficulty:     snap.Difficulty,
		KnowledgePoint: snap.KnowledgePoint,
		Score:          snap.Score,
	}
}

func (s *ExamService) toResponse(ctx context.Context, exam *model.Exam) (*dto.ExamResponse, error) {
	count, err := s.CountQuestions(ctx, exam.ID)
	if err != nil {
		return nil, fmt.Errorf("count exam questions: %w", err)
	}
	currentVersionNo := 0
	if exam.CurrentVersionID != 0 {
		if v, vErr := s.versionRepo.FindVersionByID(ctx, exam.CurrentVersionID); vErr == nil {
			currentVersionNo = v.VersionNo
		}
	}
	return &dto.ExamResponse{
		ID:               exam.ID,
		Title:            exam.Title,
		Description:      exam.Description,
		TotalScore:       exam.TotalScore,
		DurationMinutes:  exam.DurationMinutes,
		StartTime:        exam.StartTime,
		EndTime:          exam.EndTime,
		Status:           exam.Status,
		QuestionCount:    int(count),
		CurrentVersionID: exam.CurrentVersionID,
		CurrentVersionNo: currentVersionNo,
		CreatedBy:        exam.CreatedBy,
		CreatedAt:        exam.CreatedAt,
	}, nil
}

func (s *ExamService) versionResponse(ctx context.Context, v *model.ExamVersion) *dto.ExamVersionResponse {
	count, err := s.versionRepo.CountVersionQuestions(ctx, v.ID)
	if err != nil {
		count = 0
	}
	return &dto.ExamVersionResponse{
		ID:              v.ID,
		ExamID:          v.ExamID,
		VersionNo:       v.VersionNo,
		Status:          v.Status,
		TotalScore:      v.TotalScore,
		DurationMinutes: v.DurationMinutes,
		QuestionCount:   int(count),
		PublishedAt:     v.PublishedAt,
		CreatedBy:       v.CreatedBy,
		CreatedAt:       v.CreatedAt,
	}
}
