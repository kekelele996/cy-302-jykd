package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// QuestionService handles question-bank operations.
type QuestionService struct {
	baseService
	repo QuestionRepo
}

// NewQuestionService constructs QuestionService.
func NewQuestionService(repo QuestionRepo, logger *slog.Logger) *QuestionService {
	return &QuestionService{baseService: NewBaseService(logger), repo: repo}
}

// Create validates and stores a question.
func (s *QuestionService) Create(ctx context.Context, createdBy uint, req dto.QuestionRequest) (*dto.QuestionResponse, error) {
	q, err := buildQuestion(0, createdBy, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateQuestion(ctx, q); err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return questionToResponse(q), nil
}

// Update validates and updates a question.
func (s *QuestionService) Update(ctx context.Context, id uint, req dto.QuestionRequest) (*dto.QuestionResponse, error) {
	existing, err := s.repo.FindQuestionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	q, err := buildQuestion(id, existing.CreatedBy, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateQuestion(ctx, q); err != nil {
		return nil, fmt.Errorf("update question: %w", err)
	}
	return questionToResponse(q), nil
}

// Delete removes a question.
func (s *QuestionService) Delete(ctx context.Context, id uint) error {
	return s.repo.DeleteQuestion(ctx, id)
}

// Get returns one question.
func (s *QuestionService) Get(ctx context.Context, id uint) (*dto.QuestionResponse, error) {
	q, err := s.repo.FindQuestionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return questionToResponse(q), nil
}

// List returns a page of questions.
func (s *QuestionService) List(ctx context.Context, query dto.QuestionListQuery) (dto.PageResult, error) {
	questions, total, err := s.repo.ListQuestions(ctx, repository.QuestionFilter{
		Type:           query.Type,
		Difficulty:     query.Difficulty,
		KnowledgePoint: query.KnowledgePoint,
		Keyword:        query.Keyword,
	}, query.Page, query.PageSize)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("list questions: %w", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.QuestionResponse, 0, len(questions))
	for i := range questions {
		items = append(items, *questionToResponse(&questions[i]))
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ImportJSON validates and imports a JSON array of questions.
func (s *QuestionService) ImportJSON(ctx context.Context, createdBy uint, items []dto.QuestionRequest) (dto.BatchImportResult, error) {
	return s.importItems(ctx, createdBy, items)
}

// ImportExcel parses an xlsx workbook and imports its questions.
func (s *QuestionService) ImportExcel(ctx context.Context, createdBy uint, r io.Reader) (dto.BatchImportResult, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return dto.BatchImportResult{}, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return dto.BatchImportResult{}, ErrValidation
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return dto.BatchImportResult{}, fmt.Errorf("read excel rows: %w", err)
	}
	if len(rows) < 2 {
		return dto.BatchImportResult{}, ErrValidation
	}

	items := make([]dto.QuestionRequest, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if i == 0 && strings.EqualFold(strings.TrimSpace(row[0]), "type") {
			continue // tolerate a duplicate header row
		}
		req, err := rowToQuestionRequest(row)
		if err != nil {
			return dto.BatchImportResult{}, fmt.Errorf("row %d: %w", i+2, err)
		}
		items = append(items, req)
	}
	return s.importItems(ctx, createdBy, items)
}

func (s *QuestionService) importItems(ctx context.Context, createdBy uint, items []dto.QuestionRequest) (dto.BatchImportResult, error) {
	result := dto.BatchImportResult{Errors: []string{}}
	valid := make([]model.Question, 0, len(items))
	for i, req := range items {
		q, err := buildQuestion(0, createdBy, req)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("第 %d 条: %v", i+1, err))
			continue
		}
		valid = append(valid, *q)
	}
	if err := s.repo.CreateQuestionsBatch(ctx, valid); err != nil {
		return result, fmt.Errorf("create questions batch: %w", err)
	}
	result.Imported = len(valid)
	return result, nil
}

func rowToQuestionRequest(row []string) (dto.QuestionRequest, error) {
	if len(row) < 8 {
		return dto.QuestionRequest{}, fmt.Errorf("需要至少 8 列")
	}
	get := func(i int) string {
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	req := dto.QuestionRequest{
		Type:           get(0),
		Content:        get(1),
		Analysis:       get(4),
		Difficulty:     get(5),
		KnowledgePoint: get(6),
	}
	optionsRaw := get(2)
	if optionsRaw != "" {
		if err := json.Unmarshal([]byte(optionsRaw), &req.Options); err != nil {
			return dto.QuestionRequest{}, fmt.Errorf("options 列 JSON 无效: %w", err)
		}
	}
	answerRaw := get(3)
	if answerRaw == "" {
		return dto.QuestionRequest{}, fmt.Errorf("answer 列不能为空")
	}
	if err := json.Unmarshal([]byte(answerRaw), &req.Answer); err != nil {
		return dto.QuestionRequest{}, fmt.Errorf("answer 列 JSON 无效: %w", err)
	}
	var score float64
	if _, err := fmt.Sscanf(get(7), "%f", &score); err != nil {
		return dto.QuestionRequest{}, fmt.Errorf("score 列无效: %w", err)
	}
	req.Score = score
	return req, nil
}

func buildQuestion(id, createdBy uint, req dto.QuestionRequest) (*model.Question, error) {
	options, answer, err := validateQuestion(req)
	if err != nil {
		return nil, err
	}
	optionsRaw, err := marshalOptions(options)
	if err != nil {
		return nil, err
	}
	answerRaw, err := marshalAnswer(answer)
	if err != nil {
		return nil, err
	}
	return &model.Question{
		ID:             id,
		Type:           req.Type,
		Content:        strings.TrimSpace(req.Content),
		Options:        optionsRaw,
		Answer:         answerRaw,
		Analysis:       strings.TrimSpace(req.Analysis),
		Difficulty:     req.Difficulty,
		KnowledgePoint: strings.TrimSpace(req.KnowledgePoint),
		Score:          req.Score,
		CreatedBy:      createdBy,
	}, nil
}

func validateQuestion(req dto.QuestionRequest) ([]dto.Option, any, error) {
	switch req.Type {
	case constants.QuestionSingle:
		options, err := normalizeOptions(req.Options)
		if err != nil {
			return nil, nil, err
		}
		if len(options) < 2 {
			return nil, nil, fmt.Errorf("%w: 单选题至少需要 2 个选项", ErrValidation)
		}
		answer, ok := toString(req.Answer)
		if !ok || answer == "" {
			return nil, nil, fmt.Errorf("%w: 单选题答案必须是选项 key", ErrValidation)
		}
		if !optionHasKey(options, answer) {
			return nil, nil, fmt.Errorf("%w: 单选题答案不在选项中", ErrValidation)
		}
		return options, answer, nil
	case constants.QuestionMultiple:
		options, err := normalizeOptions(req.Options)
		if err != nil {
			return nil, nil, err
		}
		if len(options) < 2 {
			return nil, nil, fmt.Errorf("%w: 多选题至少需要 2 个选项", ErrValidation)
		}
		answers, ok := toStringSlice(req.Answer)
		if !ok || len(answers) == 0 {
			return nil, nil, fmt.Errorf("%w: 多选题答案必须是选项 key 数组", ErrValidation)
		}
		seen := map[string]bool{}
		for _, a := range answers {
			if !optionHasKey(options, a) {
				return nil, nil, fmt.Errorf("%w: 多选题答案 %s 不在选项中", ErrValidation, a)
			}
			if seen[a] {
				continue
			}
			seen[a] = true
		}
		return options, answers, nil
	case constants.QuestionTrueFalse:
		answer, ok := toString(req.Answer)
		if !ok || (answer != "T" && answer != "F") {
			return nil, nil, fmt.Errorf("%w: 判断题答案必须是 T 或 F", ErrValidation)
		}
		options := []dto.Option{{Key: "T", Text: "正确"}, {Key: "F", Text: "错误"}}
		return options, answer, nil
	case constants.QuestionFillBlank:
		answers, ok := toStringSlice(req.Answer)
		if !ok || len(answers) == 0 {
			return nil, nil, fmt.Errorf("%w: 填空题答案必须是数组", ErrValidation)
		}
		for _, a := range answers {
			if strings.TrimSpace(a) == "" {
				return nil, nil, fmt.Errorf("%w: 填空题答案不能为空", ErrValidation)
			}
		}
		return nil, answers, nil
	case constants.QuestionShortAnswer:
		answer, ok := toString(req.Answer)
		if !ok || strings.TrimSpace(answer) == "" {
			return nil, nil, fmt.Errorf("%w: 简答题答案必须是参考答案文本", ErrValidation)
		}
		return nil, answer, nil
	default:
		return nil, nil, fmt.Errorf("%w: 不支持的题型", ErrValidation)
	}
}

func normalizeOptions(options []dto.Option) ([]dto.Option, error) {
	seen := map[string]bool{}
	result := make([]dto.Option, 0, len(options))
	for _, opt := range options {
		key := strings.TrimSpace(opt.Key)
		text := strings.TrimSpace(opt.Text)
		if key == "" || text == "" {
			return nil, fmt.Errorf("%w: 选项 key 和内容不能为空", ErrValidation)
		}
		if seen[key] {
			return nil, fmt.Errorf("%w: 选项 key 重复: %s", ErrValidation, key)
		}
		seen[key] = true
		result = append(result, dto.Option{Key: key, Text: text})
	}
	return result, nil
}

func optionHasKey(options []dto.Option, key string) bool {
	for _, opt := range options {
		if opt.Key == key {
			return true
		}
	}
	return false
}

func questionToResponse(q *model.Question) *dto.QuestionResponse {
	options, _ := unmarshalOptions(q.Options)
	answer, _ := unmarshalAnswer(q.Answer)
	return &dto.QuestionResponse{
		ID:             q.ID,
		Type:           q.Type,
		Content:        q.Content,
		Options:        options,
		Answer:         answer,
		Analysis:       q.Analysis,
		Difficulty:     q.Difficulty,
		KnowledgePoint: q.KnowledgePoint,
		Score:          q.Score,
		CreatedBy:      q.CreatedBy,
		CreatedAt:      q.CreatedAt,
	}
}

// ObjectiveQuestionTypes returns types that can be graded automatically.
func ObjectiveQuestionTypes() map[string]bool {
	return map[string]bool{
		constants.QuestionSingle:    true,
		constants.QuestionMultiple:  true,
		constants.QuestionTrueFalse: true,
	}
}
