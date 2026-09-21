package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// AttemptService handles taking, submitting and grading exams.
type AttemptService struct {
	baseService
	examRepo    ExamRepo
	questionRepo QuestionRepo
	attemptRepo AttemptRepo
	answerRepo  AnswerRepo
	wrongRepo   WrongRepo
}

// NewAttemptService constructs AttemptService.
func NewAttemptService(
	examRepo ExamRepo,
	questionRepo QuestionRepo,
	attemptRepo AttemptRepo,
	answerRepo AnswerRepo,
	wrongRepo WrongRepo,
	logger *slog.Logger,
) *AttemptService {
	return &AttemptService{
		baseService:  NewBaseService(logger),
		examRepo:     examRepo,
		questionRepo: questionRepo,
		attemptRepo:  attemptRepo,
		answerRepo:   answerRepo,
		wrongRepo:    wrongRepo,
	}
}

// Start creates or resumes a student attempt with a shuffled paper.
func (s *AttemptService) Start(ctx context.Context, studentID, examID uint) (*dto.AttemptStartResponse, error) {
	exam, err := s.examRepo.FindExamByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	if exam.Status != constants.ExamPublished {
		return nil, ErrForbidden
	}
	now := time.Now()
	if exam.StartTime != nil && now.Before(*exam.StartTime) {
		return nil, fmt.Errorf("%w: 考试尚未开始", ErrValidation)
	}
	if exam.EndTime != nil && now.After(*exam.EndTime) {
		return nil, fmt.Errorf("%w: 考试已结束", ErrValidation)
	}

	if existing, err := s.attemptRepo.FindInProgressAttempt(ctx, examID, studentID); err == nil {
		return s.startResponse(ctx, existing, exam)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("find in progress attempt: %w", err)
	}

	items, err := s.examRepo.ListExamQuestions(ctx, examID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: 试卷没有题目", ErrValidation)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	shuffle(items, rng)

	order := make([]uint, 0, len(items))
	optionOrder := map[uint][]string{}
	for _, it := range items {
		order = append(order, it.ID)
		q, ok := s.findQuestion(ctx, it.QuestionID)
		if !ok {
			continue
		}
		if isChoiceType(q.Type) {
			options, _ := unmarshalOptions(q.Options)
			shuffle(options, rng)
			keys := make([]string, 0, len(options))
			for _, opt := range options {
				keys = append(keys, opt.Key)
			}
			optionOrder[it.ID] = keys
		}
	}

	orderRaw, _ := json.Marshal(order)
	optionRaw, _ := json.Marshal(optionOrder)
	attempt := &model.ExamAttempt{
		ExamID:        examID,
		StudentID:     studentID,
		Status:        constants.AttemptInProgress,
		StartedAt:     now,
		Deadline:      now.Add(time.Duration(exam.DurationMinutes) * time.Minute),
		QuestionOrder: string(orderRaw),
		OptionOrder:   string(optionRaw),
	}
	if err := s.attemptRepo.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return s.startResponse(ctx, attempt, exam)
}

// Current returns the student's current unfinished attempt.
func (s *AttemptService) Current(ctx context.Context, studentID, examID uint) (*dto.AttemptStartResponse, error) {
	attempt, err := s.attemptRepo.FindInProgressAttempt(ctx, examID, studentID)
	if err != nil {
		return nil, err
	}
	exam, err := s.examRepo.FindExamByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	return s.startResponse(ctx, attempt, exam)
}

// SaveAnswer persists one answer (does not auto-grade).
func (s *AttemptService) SaveAnswer(ctx context.Context, studentID, attemptID uint, req dto.AnswerSubmitRequest) error {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.StudentID != studentID {
		return ErrForbidden
	}
	if attempt.Status != constants.AttemptInProgress {
		return fmt.Errorf("%w: 试卷已提交，无法继续答题", ErrValidation)
	}
	if time.Now().After(attempt.Deadline) {
		return fmt.Errorf("%w: 考试时间已到，请交卷", ErrValidation)
	}
	eq, ok := s.findExamQuestion(ctx, attempt, req.ExamQuestionID)
	if !ok {
		return fmt.Errorf("%w: 题目不在当前试卷中", ErrValidation)
	}
	answerRaw, err := marshalAnswer(req.Answer)
	if err != nil {
		return err
	}
	marked := false
	if req.Marked != nil {
		marked = *req.Marked
	}
	answer := &model.Answer{
		AttemptID:      attemptID,
		ExamQuestionID: eq.ID,
		QuestionID:     eq.QuestionID,
		AnswerText:     answerRaw,
		Marked:         marked,
		Score:          0,
	}
	if err := s.answerRepo.SaveAnswer(ctx, answer); err != nil {
		return fmt.Errorf("save answer: %w", err)
	}
	return nil
}

// Submit finalizes an attempt, auto-grades objective questions and collects wrong answers.
func (s *AttemptService) Submit(ctx context.Context, studentID, attemptID uint) error {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.StudentID != studentID {
		return ErrForbidden
	}
	if attempt.Status != constants.AttemptInProgress {
		return ErrConflict
	}

	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return err
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}

	objectiveTotal := 0.0
	wrongItems := make([]*model.WrongQuestion, 0)
	now := time.Now()
	for _, it := range items {
		q, ok := s.findQuestion(ctx, it.QuestionID)
		if !ok {
			continue
		}
		studentAnswer := answerMap[it.ID]
		correctAnswer, _ := unmarshalAnswer(q.Answer)
		studentRaw, _ := unmarshalAnswer(studentAnswer.AnswerText)

		isObjective := ObjectiveQuestionTypes()[q.Type]
		var isCorrect *bool
		score := 0.0
		if isObjective {
			correct := studentAnswer.AnswerText != "" && isCorrectObjective(q.Type, correctAnswer, studentRaw)
			isCorrect = &correct
			if correct {
				score = it.Score
				objectiveTotal += score
			} else {
				wrongItems = append(wrongItems, &model.WrongQuestion{
					StudentID:      studentID,
					QuestionID:     q.ID,
					KnowledgePoint: q.KnowledgePoint,
					WrongCount:     1,
					LastWrongAt:    now,
					Status:         constants.WrongUnresolved,
				})
			}
		}
		saved := &model.Answer{
			AttemptID:      attemptID,
			ExamQuestionID: it.ID,
			QuestionID:     q.ID,
			AnswerText:     studentAnswer.AnswerText,
			IsCorrect:      isCorrect,
			Score:          score,
			Marked:         studentAnswer.Marked,
		}
		if err := s.answerRepo.SaveAnswer(ctx, saved); err != nil {
			return fmt.Errorf("save answer: %w", err)
		}
	}

	submittedAt := now
	attempt.Status = constants.AttemptSubmitted
	attempt.SubmittedAt = &submittedAt
	attempt.ObjectiveScore = objectiveTotal
	attempt.TotalScore = objectiveTotal
	if err := s.attemptRepo.UpdateAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	for _, w := range wrongItems {
		if err := s.wrongRepo.UpsertWrongQuestion(ctx, w); err != nil {
			return fmt.Errorf("upsert wrong question: %w", err)
		}
	}
	return nil
}

// Grade applies teacher scores to subjective answers.
func (s *AttemptService) Grade(ctx context.Context, teacherID uint, role string, attemptID uint, req dto.GradeRequest) error {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.Status != constants.AttemptSubmitted {
		return fmt.Errorf("%w: 只有已提交的试卷可以批改", ErrValidation)
	}
	exam, err := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != teacherID {
		return ErrForbidden
	}

	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return err
	}
	questionMap := make(map[uint]model.ExamQuestion, len(items))
	for _, it := range items {
		questionMap[it.ID] = it
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}

	for _, item := range req.Items {
		eq, ok := questionMap[item.ExamQuestionID]
		if !ok {
			return fmt.Errorf("%w: 题目不在该试卷中", ErrValidation)
		}
		q, ok := s.findQuestion(ctx, eq.QuestionID)
		if !ok {
			continue
		}
		if ObjectiveQuestionTypes()[q.Type] {
			continue
		}
		answer, exists := answerMap[item.ExamQuestionID]
		if !exists {
			continue
		}
		if item.Score > eq.Score {
			return fmt.Errorf("%w: 得分不能超过题目分值 %.2f", ErrValidation, eq.Score)
		}
		answer.Score = item.Score
		answer.GradedBy = teacherID
		if err := s.answerRepo.SaveAnswer(ctx, &answer); err != nil {
			return fmt.Errorf("grade answer: %w", err)
		}
		answerMap[item.ExamQuestionID] = answer
	}

	total := attempt.ObjectiveScore
	for _, a := range answerMap {
		if _, isObjective := s.questionTypeByAnswer(ctx, a); !isObjective {
			total += a.Score
		}
	}
	attempt.TotalScore = total
	if err := s.attemptRepo.UpdateAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	return nil
}

// Detail returns the full review of an attempt.
func (s *AttemptService) Detail(ctx context.Context, role string, userID, attemptID uint) (*dto.AttemptDetail, error) {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	exam, err := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleStudent && attempt.StudentID != userID {
		return nil, ErrForbidden
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}

	details, err := s.buildDetail(ctx, attempt, exam, role)
	if err != nil {
		return nil, err
	}
	return &dto.AttemptDetail{
		AttemptID:      attempt.ID,
		ExamID:         exam.ID,
		ExamTitle:      exam.Title,
		Status:         attempt.Status,
		ObjectiveScore: attempt.ObjectiveScore,
		TotalScore:     attempt.TotalScore,
		StartedAt:      attempt.StartedAt,
		SubmittedAt:    attempt.SubmittedAt,
		Deadline:       attempt.Deadline,
		Questions:      details,
	}, nil
}

// List returns the student's attempt history.
func (s *AttemptService) List(ctx context.Context, studentID uint, query dto.AttemptListQuery) (dto.PageResult, error) {
	attempts, total, err := s.attemptRepo.ListAttemptsByStudent(ctx, studentID, query.ExamID, query.Page, query.PageSize)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("list attempts: %w", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.AttemptSummary, 0, len(attempts))
	for i := range attempts {
		exam, examErr := s.examRepo.FindExamByID(ctx, attempts[i].ExamID)
		title := ""
		if examErr == nil {
			title = exam.Title
		}
		items = append(items, dto.AttemptSummary{
			AttemptID:      attempts[i].ID,
			ExamID:         attempts[i].ExamID,
			ExamTitle:      title,
			Status:         attempts[i].Status,
			ObjectiveScore: attempts[i].ObjectiveScore,
			TotalScore:     attempts[i].TotalScore,
			StartedAt:      attempts[i].StartedAt,
			SubmittedAt:    attempts[i].SubmittedAt,
		})
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Report builds score analysis with ranking.
func (s *AttemptService) Report(ctx context.Context, role string, userID, attemptID uint) (*dto.ReportResponse, error) {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	exam, err := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleStudent && attempt.StudentID != userID {
		return nil, ErrForbidden
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}

	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return nil, err
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}

	type agg struct {
		name  string
		score float64
		max   float64
		count int
	}
	aggMap := map[string]*agg{}
	objectiveCorrect := 0
	objectiveCount := 0

	for _, it := range items {
		a, ok := answerMap[it.ID]
		if !ok {
			continue
		}
		q, ok := s.findQuestion(ctx, it.QuestionID)
		if !ok {
			continue
		}
		entry, exists := aggMap[q.Type]
		if !exists {
			entry = &agg{name: questionTypeName(q.Type)}
			aggMap[q.Type] = entry
		}
		entry.score += a.Score
		entry.max += it.Score
		entry.count++
		if ObjectiveQuestionTypes()[q.Type] {
			objectiveCount++
			if a.IsCorrect != nil && *a.IsCorrect {
				objectiveCorrect++
			}
		}
	}

	type namedAgg struct {
		key   string
		entry *agg
	}
	ordered := make([]namedAgg, 0, len(aggMap))
	for key, entry := range aggMap {
		ordered = append(ordered, namedAgg{key: key, entry: entry})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	breakdown := make([]dto.TypeScore, 0, len(ordered))
	for _, item := range ordered {
		breakdown = append(breakdown, dto.TypeScore{
			Type:  item.key,
			Name:  item.entry.name,
			Score: item.entry.score,
			Max:   item.entry.max,
			Count: item.entry.count,
		})
	}

	accuracy := 0.0
	if objectiveCount > 0 {
		accuracy = float64(objectiveCorrect) / float64(objectiveCount) * 100
	}
	rank, participants := s.ranking(ctx, attempt)

	subjectiveScore := attempt.TotalScore - attempt.ObjectiveScore
	return &dto.ReportResponse{
		AttemptID:       attempt.ID,
		ExamID:          exam.ID,
		ExamTitle:       exam.Title,
		TotalScore:      attempt.TotalScore,
		ObjectiveScore:  attempt.ObjectiveScore,
		SubjectiveScore: subjectiveScore,
		Accuracy:        round2(accuracy),
		Rank:            rank,
		Participants:    participants,
		TypeBreakdown:   breakdown,
		SubmittedAt:     attempt.SubmittedAt,
	}, nil
}

// ListGrading returns submitted attempts of an exam for teacher grading.
func (s *AttemptService) ListGrading(ctx context.Context, role string, userID, examID uint) ([]dto.AttemptSummary, error) {
	exam, err := s.examRepo.FindExamByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	attempts, err := s.attemptRepo.ListAttemptsByExam(ctx, examID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.AttemptSummary, 0, len(attempts))
	for i := range attempts {
		if attempts[i].Status != constants.AttemptSubmitted {
			continue
		}
		result = append(result, dto.AttemptSummary{
			AttemptID:      attempts[i].ID,
			ExamID:         attempts[i].ExamID,
			ExamTitle:      exam.Title,
			Status:         attempts[i].Status,
			ObjectiveScore: attempts[i].ObjectiveScore,
			TotalScore:     attempts[i].TotalScore,
			StartedAt:      attempts[i].StartedAt,
			SubmittedAt:    attempts[i].SubmittedAt,
		})
	}
	return result, nil
}

func (s *AttemptService) buildDetail(ctx context.Context, attempt *model.ExamAttempt, exam *model.Exam, role string) ([]dto.AttemptQuestionDetail, error) {
	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return nil, err
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}
	order := parseOrder(attempt.QuestionOrder)
	items = orderExamQuestions(items, order)

	result := make([]dto.AttemptQuestionDetail, 0, len(items))
	for _, it := range items {
		q, ok := s.findQuestion(ctx, it.QuestionID)
		if !ok {
			continue
		}
		a := answerMap[it.ID]
		studentAnswer, _ := unmarshalAnswer(a.AnswerText)
		correctAnswer, _ := unmarshalAnswer(q.Answer)
		showCorrect := role == constants.RoleTeacher || role == constants.RoleAdmin || ObjectiveQuestionTypes()[q.Type]
		if !showCorrect {
			correctAnswer = nil
		}
		var isCorrect *bool
		if a.IsCorrect != nil {
			val := *a.IsCorrect
			isCorrect = &val
		}
		result = append(result, dto.AttemptQuestionDetail{
			ExamQuestionID: it.ID,
			Type:           q.Type,
			Content:        q.Content,
			Options:        mustOptions(q.Options),
			StudentAnswer:  studentAnswer,
			CorrectAnswer:  correctAnswer,
			IsCorrect:      isCorrect,
			Score:          a.Score,
			MaxScore:       it.Score,
			Analysis:       q.Analysis,
			Marked:         a.Marked,
			Graded:         a.GradedBy != 0,
		})
	}
	return result, nil
}

func (s *AttemptService) startResponse(ctx context.Context, attempt *model.ExamAttempt, exam *model.Exam) (*dto.AttemptStartResponse, error) {
	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return nil, err
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}
	order := parseOrder(attempt.QuestionOrder)
	items = orderExamQuestions(items, order)
	optionOrder := parseOptionOrder(attempt.OptionOrder)

	views := make([]dto.ExamQuestionView, 0, len(items))
	for _, it := range items {
		q, ok := s.findQuestion(ctx, it.QuestionID)
		if !ok {
			continue
		}
		options, _ := unmarshalOptions(q.Options)
		if keys, ok := optionOrder[it.ID]; ok {
			options = reorderOptions(options, keys)
		}
		a := answerMap[it.ID]
		studentAnswer, _ := unmarshalAnswer(a.AnswerText)
		views = append(views, dto.ExamQuestionView{
			ExamQuestionID: it.ID,
			Type:           q.Type,
			Content:        q.Content,
			Options:        options,
			Score:          it.Score,
			Marked:         a.Marked,
			Answer:         studentAnswer,
		})
	}
	return &dto.AttemptStartResponse{
		AttemptID:       attempt.ID,
		ExamID:          exam.ID,
		Title:           exam.Title,
		DurationMinutes: exam.DurationMinutes,
		TotalScore:      exam.TotalScore,
		StartedAt:       attempt.StartedAt,
		Deadline:        attempt.Deadline,
		Questions:       views,
	}, nil
}

func (s *AttemptService) findQuestion(ctx context.Context, id uint) (model.Question, bool) {
	m, err := s.questionRepo.FindQuestionsByIDs(ctx, []uint{id})
	if err != nil {
		return model.Question{}, false
	}
	q, ok := m[id]
	return q, ok
}

func (s *AttemptService) findExamQuestion(ctx context.Context, attempt *model.ExamAttempt, eqID uint) (model.ExamQuestion, bool) {
	items, err := s.examRepo.ListExamQuestions(ctx, attempt.ExamID)
	if err != nil {
		return model.ExamQuestion{}, false
	}
	for _, it := range items {
		if it.ID == eqID {
			return it, true
		}
	}
	return model.ExamQuestion{}, false
}

func (s *AttemptService) questionTypeByAnswer(ctx context.Context, a model.Answer) (string, bool) {
	q, ok := s.findQuestion(ctx, a.QuestionID)
	if !ok {
		return "", true
	}
	return q.Type, ObjectiveQuestionTypes()[q.Type]
}

func (s *AttemptService) ranking(ctx context.Context, attempt *model.ExamAttempt) (int, int) {
	attempts, err := s.attemptRepo.ListAttemptsByExam(ctx, attempt.ExamID)
	if err != nil {
		return 0, 0
	}
	submitted := make([]model.ExamAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a.Status == constants.AttemptSubmitted {
			submitted = append(submitted, a)
		}
	}
	sort.Slice(submitted, func(i, j int) bool {
		if submitted[i].TotalScore != submitted[j].TotalScore {
			return submitted[i].TotalScore > submitted[j].TotalScore
		}
		return submitted[i].SubmittedAt.Before(*submitted[j].SubmittedAt)
	})
	for i, a := range submitted {
		if a.ID == attempt.ID {
			return i + 1, len(submitted)
		}
	}
	return 0, len(submitted)
}

func parseOrder(raw string) []uint {
	var order []uint
	_ = json.Unmarshal([]byte(raw), &order)
	return order
}

func parseOptionOrder(raw string) map[uint][]string {
	result := map[uint][]string{}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func orderExamQuestions(items []model.ExamQuestion, order []uint) []model.ExamQuestion {
	if len(order) == 0 {
		return items
	}
	byID := make(map[uint]model.ExamQuestion, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	result := make([]model.ExamQuestion, 0, len(items))
	for _, id := range order {
		if it, ok := byID[id]; ok {
			result = append(result, it)
		}
	}
	return result
}

func reorderOptions(options []dto.Option, keys []string) []dto.Option {
	if len(keys) == 0 {
		return options
	}
	byKey := make(map[string]dto.Option, len(options))
	for _, opt := range options {
		byKey[opt.Key] = opt
	}
	result := make([]dto.Option, 0, len(keys))
	for _, key := range keys {
		if opt, ok := byKey[key]; ok {
			result = append(result, opt)
		}
	}
	return result
}

func mustOptions(raw string) []dto.Option {
	options, _ := unmarshalOptions(raw)
	return options
}

func isChoiceType(qtype string) bool {
	return qtype == constants.QuestionSingle || qtype == constants.QuestionMultiple || qtype == constants.QuestionTrueFalse
}

func isCorrectObjective(qtype string, correct, student any) bool {
	switch qtype {
	case constants.QuestionSingle, constants.QuestionTrueFalse:
		c, ok1 := toString(correct)
		s, ok2 := toString(student)
		return ok1 && ok2 && strings.EqualFold(strings.TrimSpace(c), strings.TrimSpace(s))
	case constants.QuestionMultiple:
		c, ok1 := toStringSlice(correct)
		s, ok2 := toStringSlice(student)
		if !ok1 || !ok2 || len(c) != len(s) {
			return false
		}
		set := make(map[string]bool, len(c))
		for _, v := range c {
			set[strings.TrimSpace(v)] = true
		}
		for _, v := range s {
			if !set[strings.TrimSpace(v)] {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func questionTypeName(qtype string) string {
	switch qtype {
	case constants.QuestionSingle:
		return "单选题"
	case constants.QuestionMultiple:
		return "多选题"
	case constants.QuestionTrueFalse:
		return "判断题"
	case constants.QuestionFillBlank:
		return "填空题"
	case constants.QuestionShortAnswer:
		return "简答题"
	default:
		return qtype
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
