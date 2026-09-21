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
)

// WrongQuestionService manages the wrong-question book and practice.
type WrongQuestionService struct {
	baseService
	wrongRepo    WrongRepo
	questionRepo QuestionRepo
}

// NewWrongQuestionService constructs WrongQuestionService.
func NewWrongQuestionService(wrongRepo WrongRepo, questionRepo QuestionRepo, logger *slog.Logger) *WrongQuestionService {
	return &WrongQuestionService{baseService: NewBaseService(logger), wrongRepo: wrongRepo, questionRepo: questionRepo}
}

// List returns a page of a student's wrong questions.
func (s *WrongQuestionService) List(ctx context.Context, studentID uint, query dto.WrongQuestionListQuery) (dto.PageResult, error) {
	records, total, err := s.wrongRepo.ListWrongQuestions(ctx, studentID, query.KnowledgePoint, query.Page, query.PageSize)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("list wrong questions: %w", err)
	}
	ids := make([]uint, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDs(ctx, ids)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("find questions by ids: %w", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.WrongQuestionItem, 0, len(records))
	for _, r := range records {
		q, ok := questionMap[r.QuestionID]
		if !ok {
			continue
		}
		items = append(items, dto.WrongQuestionItem{
			ID:             r.ID,
			QuestionID:     r.QuestionID,
			KnowledgePoint: r.KnowledgePoint,
			WrongCount:     r.WrongCount,
			Status:         r.Status,
			LastWrongAt:    r.LastWrongAt,
			Question:       *questionToResponse(&q),
		})
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Delete removes a wrong-question record.
func (s *WrongQuestionService) Delete(ctx context.Context, studentID, id uint) error {
	return s.wrongRepo.DeleteWrongQuestion(ctx, id, studentID)
}

// Practice returns a shuffled set of unresolved wrong questions.
func (s *WrongQuestionService) Practice(ctx context.Context, studentID uint) (*dto.PracticeStartResponse, error) {
	records, _, err := s.wrongRepo.ListWrongQuestions(ctx, studentID, "", 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("list wrong questions: %w", err)
	}
	unresolved := make([]model.WrongQuestion, 0, len(records))
	for _, r := range records {
		if r.Status == constants.WrongUnresolved {
			unresolved = append(unresolved, r)
		}
	}
	ids := make([]uint, 0, len(unresolved))
	for _, r := range unresolved {
		ids = append(ids, r.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find questions by ids: %w", err)
	}
	questions := make([]model.Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := questionMap[id]; ok {
			questions = append(questions, q)
		}
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	shuffle(questions, rng)

	items := make([]dto.PracticeQuestion, 0, len(questions))
	for i := range questions {
		q := questions[i]
		options, _ := unmarshalOptions(q.Options)
		if isChoiceType(q.Type) {
			shuffle(options, rng)
		}
		items = append(items, dto.PracticeQuestion{
			QuestionID:     q.ID,
			Type:           q.Type,
			Content:        q.Content,
			Options:        options,
			Score:          q.Score,
			KnowledgePoint: q.KnowledgePoint,
		})
	}
	return &dto.PracticeStartResponse{Questions: items}, nil
}

// SubmitPractice grades a practice set and updates wrong-question status.
func (s *WrongQuestionService) SubmitPractice(ctx context.Context, studentID uint, req dto.PracticeAnswerRequest) (*dto.PracticeResultResponse, error) {
	ids := make([]uint, 0, len(req.Answers))
	for _, a := range req.Answers {
		ids = append(ids, a.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find questions by ids: %w", err)
	}
	records, _, err := s.wrongRepo.ListWrongQuestions(ctx, studentID, "", 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("list wrong questions: %w", err)
	}
	recordByQuestion := make(map[uint]model.WrongQuestion, len(records))
	for _, r := range records {
		recordByQuestion[r.QuestionID] = r
	}

	result := &dto.PracticeResultResponse{Items: make([]dto.PracticeResultItem, 0, len(req.Answers))}
	for _, a := range req.Answers {
		q, ok := questionMap[a.QuestionID]
		if !ok {
			continue
		}
		if !ObjectiveQuestionTypes()[q.Type] {
			continue
		}
		correctAnswer, _ := unmarshalAnswer(q.Answer)
		correct := isCorrectObjective(q.Type, correctAnswer, a.Answer)
		item := dto.PracticeResultItem{QuestionID: q.ID, Correct: correct, Score: 0}
		if correct {
			item.Score = q.Score
			if record, exists := recordByQuestion[q.ID]; exists {
				if err := s.wrongRepo.MarkWrongQuestionResolved(ctx, record.ID, studentID); err != nil {
					return nil, fmt.Errorf("resolve wrong question: %w", err)
				}
			}
		} else {
			w := &model.WrongQuestion{
				StudentID:      studentID,
				QuestionID:     q.ID,
				KnowledgePoint: q.KnowledgePoint,
				WrongCount:     1,
				LastWrongAt:    time.Now(),
				Status:         constants.WrongUnresolved,
			}
			if err := s.wrongRepo.UpsertWrongQuestion(ctx, w); err != nil {
				return nil, fmt.Errorf("upsert wrong question: %w", err)
			}
		}
		result.Items = append(result.Items, item)
		result.Total++
		if correct {
			result.Correct++
		}
	}
	return result, nil
}
