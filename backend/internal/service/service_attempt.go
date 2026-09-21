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
)

// AttemptService handles taking, submitting and grading exams. All paper
// content is read from the frozen version snapshots pinned to each attempt,
// so concurrent question-bank changes never alter what a student sees or how
// a paper is graded.
type AttemptService struct {
	baseService
	examRepo    ExamRepo
	versionRepo VersionRepo
	attemptRepo AttemptRepo
	answerRepo  AnswerRepo
	wrongRepo   WrongRepo
}

// NewAttemptService constructs AttemptService.
func NewAttemptService(
	examRepo ExamRepo,
	versionRepo VersionRepo,
	attemptRepo AttemptRepo,
	answerRepo AnswerRepo,
	wrongRepo WrongRepo,
	logger *slog.Logger,
) *AttemptService {
	return &AttemptService{
		baseService:  NewBaseService(logger),
		examRepo:     examRepo,
		versionRepo:  versionRepo,
		attemptRepo:  attemptRepo,
		answerRepo:   answerRepo,
		wrongRepo:    wrongRepo,
	}
}

// Start creates or resumes a student attempt with a shuffled frozen paper.
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
	} else if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("find in progress attempt: %w", err)
	}

	// New attempts always bind the currently active frozen version.
	version, err := s.versionRepo.FindCurrentVersion(ctx, examID)
	if err != nil {
		return nil, fmt.Errorf("find current version: %w", err)
	}
	if version.Status != constants.VersionPublished {
		return nil, fmt.Errorf("%w: 当前没有已发布的试卷版本", ErrValidation)
	}
	items, err := s.versionRepo.ListVersionQuestions(ctx, version.ID)
	if err != nil {
		return nil, fmt.Errorf("list version questions: %w", err)
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
		if isChoiceType(it.Type) {
			options, _ := unmarshalOptions(it.Options)
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
		VersionID:     version.ID,
		StudentID:     studentID,
		Status:        constants.AttemptInProgress,
		StartedAt:     now,
		Deadline:      now.Add(time.Duration(version.DurationMinutes) * time.Minute),
		QuestionOrder: string(orderRaw),
		OptionOrder:   string(optionRaw),
	}
	if err := s.attemptRepo.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return s.startResponse(ctx, attempt, exam)
}

// Current returns the student's current unfinished attempt. The response is
// rebuilt from the same frozen version every time, so refreshing the page
// yields identical content.
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
	snap, ok, err := s.findVersionQuestion(ctx, attempt, req.ExamQuestionID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: 题目不在当前试卷版本中", ErrValidation)
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
		ExamQuestionID: snap.ID,
		QuestionID:     snap.QuestionID,
		AnswerText:     answerRaw,
		Marked:         marked,
		Score:          0,
	}
	if err := s.answerRepo.SaveAnswer(ctx, answer); err != nil {
		return fmt.Errorf("save answer: %w", err)
	}
	return nil
}

// Submit finalizes an attempt, auto-grades objective questions and collects
// wrong answers. Duplicate or concurrent submission takes effect only once.
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

	items, err := s.attemptPaper(ctx, attempt)
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
		studentAnswer := answerMap[it.ID]
		correctAnswer, _ := unmarshalAnswer(it.Answer)
		studentRaw, _ := unmarshalAnswer(studentAnswer.AnswerText)

		isObjective := ObjectiveQuestionTypes()[it.Type]
		var isCorrect *bool
		score := 0.0
		if isObjective {
			correct := studentAnswer.AnswerText != "" && isCorrectObjective(it.Type, correctAnswer, studentRaw)
			isCorrect = &correct
			if correct {
				score = it.Score
				objectiveTotal += score
			} else {
				wrongItems = append(wrongItems, &model.WrongQuestion{
					StudentID:      studentID,
					QuestionID:     it.QuestionID,
					KnowledgePoint: it.KnowledgePoint,
					WrongCount:     1,
					LastWrongAt:    now,
					Status:         constants.WrongUnresolved,
				})
			}
		}
		saved := &model.Answer{
			AttemptID:      attemptID,
			ExamQuestionID: it.ID,
			QuestionID:     it.QuestionID,
			AnswerText:     studentAnswer.AnswerText,
			IsCorrect:      isCorrect,
			Score:          score,
			Marked:         studentAnswer.Marked,
		}
		if err := s.answerRepo.SaveAnswer(ctx, saved); err != nil {
			return fmt.Errorf("save answer: %w", err)
		}
	}

	// CAS: a concurrent submit (e.g. timeout + manual) only commits once.
	if err := s.attemptRepo.SubmitAttemptCAS(ctx, attemptID, now, objectiveTotal, objectiveTotal); err != nil {
		if errors.Is(err, ErrConflict) {
			return ErrConflict
		}
		return fmt.Errorf("submit attempt: %w", err)
	}
	for _, w := range wrongItems {
		if err := s.wrongRepo.UpsertWrongQuestion(ctx, w); err != nil {
			return fmt.Errorf("upsert wrong question: %w", err)
		}
	}
	return nil
}

// Grade applies teacher scores to subjective answers against the frozen paper.
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

	items, err := s.attemptPaper(ctx, attempt)
	if err != nil {
		return err
	}
	snapshotMap := make(map[uint]model.ExamVersionQuestion, len(items))
	for _, it := range items {
		snapshotMap[it.ID] = it
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
		snap, ok := snapshotMap[item.ExamQuestionID]
		if !ok {
			return fmt.Errorf("%w: 题目不在该试卷版本中", ErrValidation)
		}
		if ObjectiveQuestionTypes()[snap.Type] {
			continue
		}
		answer, exists := answerMap[item.ExamQuestionID]
		if !exists {
			continue
		}
		if item.Score > snap.Score {
			return fmt.Errorf("%w: 得分不能超过题目分值 %.2f", ErrValidation, snap.Score)
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
		if snap, ok := snapshotMap[a.ExamQuestionID]; ok && !ObjectiveQuestionTypes()[snap.Type] {
			total += a.Score
		}
	}
	attempt.TotalScore = total
	if err := s.attemptRepo.UpdateAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	return nil
}

// Detail returns the full review of an attempt, rendered from the frozen
// version pinned when the attempt started.
func (s *AttemptService) Detail(ctx context.Context, role string, userID, attemptID uint) (*dto.AttemptDetail, error) {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	exam, examErr := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if examErr != nil && !errors.Is(examErr, ErrNotFound) {
		return nil, examErr
	}
	if err := s.checkAttemptAccess(ctx, role, userID, attempt, exam); err != nil {
		return nil, err
	}

	details, err := s.buildDetail(ctx, attempt, role)
	if err != nil {
		return nil, err
	}
	version, _ := s.versionRepo.FindVersionByID(ctx, attempt.VersionID)
	versionNo := 0
	if version != nil {
		versionNo = version.VersionNo
	}
	return &dto.AttemptDetail{
		AttemptID:      attempt.ID,
		ExamID:         attempt.ExamID,
		VersionID:      attempt.VersionID,
		VersionNo:      versionNo,
		ExamTitle:      examTitle(exam, attempt.ExamID),
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
		summary, sumErr := s.toSummary(ctx, &attempts[i])
		if sumErr != nil {
			return dto.PageResult{}, sumErr
		}
		items = append(items, *summary)
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Report builds score analysis with ranking from the frozen paper.
func (s *AttemptService) Report(ctx context.Context, role string, userID, attemptID uint) (*dto.ReportResponse, error) {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	exam, examErr := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if examErr != nil && !errors.Is(examErr, ErrNotFound) {
		return nil, examErr
	}
	if err := s.checkAttemptAccess(ctx, role, userID, attempt, exam); err != nil {
		return nil, err
	}

	items, err := s.attemptPaper(ctx, attempt)
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
		entry, exists := aggMap[it.Type]
		if !exists {
			entry = &agg{name: questionTypeName(it.Type)}
			aggMap[it.Type] = entry
		}
		entry.score += a.Score
		entry.max += it.Score
		entry.count++
		if ObjectiveQuestionTypes()[it.Type] {
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

	version, _ := s.versionRepo.FindVersionByID(ctx, attempt.VersionID)
	versionNo := 0
	if version != nil {
		versionNo = version.VersionNo
	}
	subjectiveScore := attempt.TotalScore - attempt.ObjectiveScore
	return &dto.ReportResponse{
		AttemptID:       attempt.ID,
		ExamID:          attempt.ExamID,
		VersionID:       attempt.VersionID,
		VersionNo:       versionNo,
		ExamTitle:       examTitle(exam, attempt.ExamID),
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
		summary, sumErr := s.toSummary(ctx, &attempts[i])
		if sumErr != nil {
			return nil, sumErr
		}
		result = append(result, *summary)
	}
	return result, nil
}

func (s *AttemptService) buildDetail(ctx context.Context, attempt *model.ExamAttempt, role string) ([]dto.AttemptQuestionDetail, error) {
	items, err := s.attemptPaper(ctx, attempt)
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
	items = orderVersionQuestions(items, order)

	result := make([]dto.AttemptQuestionDetail, 0, len(items))
	for _, it := range items {
		a := answerMap[it.ID]
		studentAnswer, _ := unmarshalAnswer(a.AnswerText)
		correctAnswer, _ := unmarshalAnswer(it.Answer)
		showCorrect := role == constants.RoleTeacher || role == constants.RoleAdmin || ObjectiveQuestionTypes()[it.Type]
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
			Type:           it.Type,
			Content:        it.Content,
			Options:        mustOptions(it.Options),
			StudentAnswer:  studentAnswer,
			CorrectAnswer:  correctAnswer,
			IsCorrect:      isCorrect,
			Score:          a.Score,
			MaxScore:       it.Score,
			Analysis:       it.Analysis,
			Marked:         a.Marked,
			Graded:         a.GradedBy != 0,
		})
	}
	return result, nil
}

func (s *AttemptService) startResponse(ctx context.Context, attempt *model.ExamAttempt, exam *model.Exam) (*dto.AttemptStartResponse, error) {
	version, items, err := s.attemptVersion(ctx, attempt)
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
	items = orderVersionQuestions(items, order)
	optionOrder := parseOptionOrder(attempt.OptionOrder)

	views := make([]dto.ExamQuestionView, 0, len(items))
	for _, it := range items {
		options, _ := unmarshalOptions(it.Options)
		if keys, ok := optionOrder[it.ID]; ok {
			options = reorderOptions(options, keys)
		}
		a := answerMap[it.ID]
		studentAnswer, _ := unmarshalAnswer(a.AnswerText)
		views = append(views, dto.ExamQuestionView{
			ExamQuestionID: it.ID,
			Type:           it.Type,
			Content:        it.Content,
			Options:        options,
			Score:          it.Score,
			Marked:         a.Marked,
			Answer:         studentAnswer,
		})
	}
	return &dto.AttemptStartResponse{
		AttemptID:       attempt.ID,
		ExamID:          attempt.ExamID,
		VersionID:       version.ID,
		VersionNo:       version.VersionNo,
		Title:           examTitle(exam, attempt.ExamID),
		DurationMinutes: version.DurationMinutes,
		TotalScore:      version.TotalScore,
		StartedAt:       attempt.StartedAt,
		Deadline:        attempt.Deadline,
		Questions:       views,
	}, nil
}

// attemptPaper loads the frozen version and its snapshots bound to an attempt.
func (s *AttemptService) attemptPaper(ctx context.Context, attempt *model.ExamAttempt) ([]model.ExamVersionQuestion, error) {
	_, items, err := s.attemptVersion(ctx, attempt)
	return items, err
}

func (s *AttemptService) attemptVersion(ctx context.Context, attempt *model.ExamAttempt) (*model.ExamVersion, []model.ExamVersionQuestion, error) {
	versionID := attempt.VersionID
	if versionID == 0 {
		// Backfill safety for rows migrated before versions existed.
		if current, err := s.versionRepo.FindCurrentVersion(ctx, attempt.ExamID); err == nil {
			versionID = current.ID
		}
	}
	version, err := s.versionRepo.FindVersionByID(ctx, versionID)
	if err != nil {
		return nil, nil, fmt.Errorf("find attempt version: %w", err)
	}
	items, err := s.versionRepo.ListVersionQuestions(ctx, version.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list version questions: %w", err)
	}
	return version, items, nil
}

func (s *AttemptService) findVersionQuestion(ctx context.Context, attempt *model.ExamAttempt, eqID uint) (model.ExamVersionQuestion, bool, error) {
	items, err := s.attemptPaper(ctx, attempt)
	if err != nil {
		return model.ExamVersionQuestion{}, false, err
	}
	for _, it := range items {
		if it.ID == eqID {
			return it, true, nil
		}
	}
	return model.ExamVersionQuestion{}, false, nil
}

func (s *AttemptService) toSummary(ctx context.Context, a *model.ExamAttempt) (*dto.AttemptSummary, error) {
	title := ""
	if exam, err := s.examRepo.FindExamByID(ctx, a.ExamID); err == nil {
		title = exam.Title
	}
	versionNo := 0
	if a.VersionID != 0 {
		if v, err := s.versionRepo.FindVersionByID(ctx, a.VersionID); err == nil {
			versionNo = v.VersionNo
		}
	}
	return &dto.AttemptSummary{
		AttemptID:      a.ID,
		ExamID:         a.ExamID,
		VersionID:      a.VersionID,
		VersionNo:      versionNo,
		ExamTitle:      title,
		Status:         a.Status,
		ObjectiveScore: a.ObjectiveScore,
		TotalScore:     a.TotalScore,
		StartedAt:      a.StartedAt,
		SubmittedAt:    a.SubmittedAt,
	}, nil
}

// checkAttemptAccess enforces ownership. A missing exam (deleted by staff)
// still allows the owning student or an admin to review historical results.
func (s *AttemptService) checkAttemptAccess(ctx context.Context, role string, userID uint, attempt *model.ExamAttempt, exam *model.Exam) error {
	switch role {
	case constants.RoleStudent:
		if attempt.StudentID != userID {
			return ErrForbidden
		}
	case constants.RoleTeacher:
		if exam == nil || exam.CreatedBy != userID {
			return ErrForbidden
		}
	case constants.RoleAdmin:
		return nil
	default:
		return ErrForbidden
	}
	return nil
}

func examTitle(exam *model.Exam, examID uint) string {
	if exam != nil {
		return exam.Title
	}
	return fmt.Sprintf("考试 #%d（已删除）", examID)
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

func orderVersionQuestions(items []model.ExamVersionQuestion, order []uint) []model.ExamVersionQuestion {
	if len(order) == 0 {
		return items
	}
	byID := make(map[uint]model.ExamVersionQuestion, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	result := make([]model.ExamVersionQuestion, 0, len(items))
	for _, id := range order {
		if it, ok := byID[id]; ok {
			result = append(result, it)
		}
	}
	if len(result) == 0 {
		return items
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
