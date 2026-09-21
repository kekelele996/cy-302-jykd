package service

import (
	"context"
	"encoding/json"
	"errors"
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
// Every paper read goes through the PaperVersion snapshot bound to the attempt,
// so question bank edits/withdrawals never change what an in-progress or
// historical exam displays, grades against or reports.
type AttemptService struct {
	baseService
	examRepo     ExamRepo
	paperRepo    PaperRepo
	questionRepo QuestionRepo
	attemptRepo  AttemptRepo
	answerRepo   AnswerRepo
	wrongRepo    WrongRepo
}

// NewAttemptService constructs AttemptService.
func NewAttemptService(
	examRepo ExamRepo,
	paperRepo PaperRepo,
	questionRepo QuestionRepo,
	attemptRepo AttemptRepo,
	answerRepo AnswerRepo,
	wrongRepo WrongRepo,
	logger *slog.Logger,
) *AttemptService {
	return &AttemptService{
		baseService:  NewBaseService(logger),
		examRepo:     examRepo,
		paperRepo:    paperRepo,
		questionRepo: questionRepo,
		attemptRepo:  attemptRepo,
		answerRepo:   answerRepo,
		wrongRepo:    wrongRepo,
	}
}

// Start creates or resumes a student attempt with a shuffled paper.
// New attempts bind to the exam's current frozen version; resumed attempts keep
// the version they originally started with, even if a newer version exists.
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
		return nil, errWrapf("%w: 考试尚未开始", ErrValidation)
	}
	if exam.EndTime != nil && now.After(*exam.EndTime) {
		return nil, errWrapf("%w: 考试已结束", ErrValidation)
	}

	if existing, err := s.attemptRepo.FindInProgressAttempt(ctx, examID, studentID); err == nil {
		return s.startResponse(ctx, existing, exam)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, errWrap("find in progress attempt", err)
	}

	// Resolve and lock the frozen version the attempt will be bound to.
	version, err := s.paperRepo.FindCurrentPaperVersion(ctx, examID)
	if err != nil {
		return nil, errWrap("resolve current paper version", err)
	}
	items, err := s.paperRepo.ListPaperVersionQuestions(ctx, version.ID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errWrapf("%w: 试卷没有题目", ErrValidation)
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
		ExamID:         examID,
		PaperVersionID: version.ID,
		StudentID:      studentID,
		Status:         constants.AttemptInProgress,
		StartedAt:      now,
		Deadline:       now.Add(time.Duration(exam.DurationMinutes) * time.Minute),
		QuestionOrder:  string(orderRaw),
		OptionOrder:    string(optionRaw),
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
		return errWrapf("%w: 试卷已提交，无法继续答题", ErrValidation)
	}
	if time.Now().After(attempt.Deadline) {
		return errWrapf("%w: 考试时间已到，请交卷", ErrValidation)
	}
	fq, ok, err := s.findVersionQuestion(ctx, attempt, req.ExamQuestionID)
	if err != nil {
		return err
	}
	if !ok {
		return errWrapf("%w: 题目不在当前试卷中", ErrValidation)
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
		ExamQuestionID: fq.ID,
		QuestionID:     fq.QuestionID,
		AnswerText:     answerRaw,
		Marked:         marked,
		Score:          0,
	}
	if err := s.answerRepo.SaveAnswer(ctx, answer); err != nil {
		return errWrap("save answer", err)
	}
	return nil
}

// Submit finalizes an attempt, auto-grades objective questions and collects
// wrong answers. Grading compares against the frozen reference answer captured
// in the attempt's paper version.
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

	frozen, err := s.versionQuestions(ctx, attempt)
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
	for i := range frozen {
		it := &frozen[i]
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
			return errWrap("save answer", err)
		}
	}

	submittedAt := now
	attempt.Status = constants.AttemptSubmitted
	attempt.SubmittedAt = &submittedAt
	attempt.ObjectiveScore = objectiveTotal
	attempt.TotalScore = objectiveTotal
	if err := s.attemptRepo.UpdateAttempt(ctx, attempt); err != nil {
		return errWrap("update attempt", err)
	}
	for _, w := range wrongItems {
		if err := s.wrongRepo.UpsertWrongQuestion(ctx, w); err != nil {
			return errWrap("upsert wrong question", err)
		}
	}
	return nil
}

// Grade applies teacher scores to subjective answers. Score caps come from the
// frozen per-question score captured in the attempt's version.
func (s *AttemptService) Grade(ctx context.Context, teacherID uint, role string, attemptID uint, req dto.GradeRequest) error {
	attempt, err := s.attemptRepo.FindAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.Status != constants.AttemptSubmitted {
		return errWrapf("%w: 只有已提交的试卷可以批改", ErrValidation)
	}
	exam, err := s.examRepo.FindExamByID(ctx, attempt.ExamID)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != teacherID {
		return ErrForbidden
	}

	frozen, err := s.versionQuestions(ctx, attempt)
	if err != nil {
		return err
	}
	questionMap := make(map[uint]model.PaperVersionQuestion, len(frozen))
	for i := range frozen {
		questionMap[frozen[i].ID] = frozen[i]
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
		fq, ok := questionMap[item.ExamQuestionID]
		if !ok {
			return errWrapf("%w: 题目不在该试卷中", ErrValidation)
		}
		if ObjectiveQuestionTypes()[fq.Type] {
			continue
		}
		answer, exists := answerMap[item.ExamQuestionID]
		if !exists {
			continue
		}
		if item.Score > fq.Score {
			return errWrapf("%w: 得分不能超过题目分值 %.2f", ErrValidation, fq.Score)
		}
		answer.Score = item.Score
		answer.GradedBy = teacherID
		if err := s.answerRepo.SaveAnswer(ctx, &answer); err != nil {
			return errWrap("grade answer", err)
		}
		answerMap[item.ExamQuestionID] = answer
	}

	total := attempt.ObjectiveScore
	for _, a := range answerMap {
		if fq, ok := questionMap[a.ExamQuestionID]; ok {
			if !ObjectiveQuestionTypes()[fq.Type] {
				total += a.Score
			}
		}
	}
	attempt.TotalScore = total
	if err := s.attemptRepo.UpdateAttempt(ctx, attempt); err != nil {
		return errWrap("update attempt", err)
	}
	return nil
}

// Detail returns the full review of an attempt. Content, options, reference
// answers and max scores all come from the version frozen at attempt start.
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

	version, details, err := s.buildDetail(ctx, attempt, role)
	if err != nil {
		return nil, err
	}
	return &dto.AttemptDetail{
		AttemptID:      attempt.ID,
		ExamID:         exam.ID,
		PaperVersionID: attempt.PaperVersionID,
		VersionNo:      version.VersionNo,
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
		return dto.PageResult{}, errWrap("list attempts", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.AttemptSummary, 0, len(attempts))
	for i := range attempts {
		exam, examErr := s.examRepo.FindExamByID(ctx, attempts[i].ExamID)
		title := ""
		if examErr == nil {
			title = exam.Title
		}
		versionNo := 0
		if attempts[i].PaperVersionID != 0 {
			if v, vErr := s.paperRepo.FindPaperVersionByID(ctx, attempts[i].PaperVersionID); vErr == nil {
				versionNo = v.VersionNo
			}
		}
		items = append(items, dto.AttemptSummary{
			AttemptID:      attempts[i].ID,
			ExamID:         attempts[i].ExamID,
			PaperVersionID: attempts[i].PaperVersionID,
			VersionNo:      versionNo,
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

// Report builds score analysis with ranking, based on the frozen version.
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

	frozen, err := s.versionQuestions(ctx, attempt)
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

	for i := range frozen {
		it := &frozen[i]
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

	versionNo := 0
	if attempt.PaperVersionID != 0 {
		if v, vErr := s.paperRepo.FindPaperVersionByID(ctx, attempt.PaperVersionID); vErr == nil {
			versionNo = v.VersionNo
		}
	}
	subjectiveScore := attempt.TotalScore - attempt.ObjectiveScore
	return &dto.ReportResponse{
		AttemptID:       attempt.ID,
		ExamID:          exam.ID,
		PaperVersionID:  attempt.PaperVersionID,
		VersionNo:       versionNo,
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
		versionNo := 0
		if attempts[i].PaperVersionID != 0 {
			if v, vErr := s.paperRepo.FindPaperVersionByID(ctx, attempts[i].PaperVersionID); vErr == nil {
				versionNo = v.VersionNo
			}
		}
		result = append(result, dto.AttemptSummary{
			AttemptID:      attempts[i].ID,
			ExamID:         attempts[i].ExamID,
			PaperVersionID: attempts[i].PaperVersionID,
			VersionNo:      versionNo,
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

func (s *AttemptService) buildDetail(ctx context.Context, attempt *model.ExamAttempt, role string) (*model.PaperVersion, []dto.AttemptQuestionDetail, error) {
	version, frozen, err := s.orderedVersionQuestions(ctx, attempt)
	if err != nil {
		return nil, nil, err
	}
	answers, err := s.answerRepo.ListAnswersByAttempt(ctx, attempt.ID)
	if err != nil {
		return nil, nil, err
	}
	answerMap := make(map[uint]model.Answer, len(answers))
	for _, a := range answers {
		answerMap[a.ExamQuestionID] = a
	}

	result := make([]dto.AttemptQuestionDetail, 0, len(frozen))
	for i := range frozen {
		it := &frozen[i]
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
	return version, result, nil
}

func (s *AttemptService) startResponse(ctx context.Context, attempt *model.ExamAttempt, exam *model.Exam) (*dto.AttemptStartResponse, error) {
	version, items, err := s.orderedVersionQuestions(ctx, attempt)
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
	optionOrder := parseOptionOrder(attempt.OptionOrder)

	views := make([]dto.ExamQuestionView, 0, len(items))
	for i := range items {
		it := &items[i]
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
		ExamID:          exam.ID,
		PaperVersionID:  attempt.PaperVersionID,
		VersionNo:       version.VersionNo,
		Title:           exam.Title,
		DurationMinutes: exam.DurationMinutes,
		TotalScore:      version.TotalScore,
		StartedAt:       attempt.StartedAt,
		Deadline:        attempt.Deadline,
		Questions:       views,
	}, nil
}

// versionQuestions loads the frozen questions bound to an attempt.
func (s *AttemptService) versionQuestions(ctx context.Context, attempt *model.ExamAttempt) ([]model.PaperVersionQuestion, error) {
	if attempt.PaperVersionID == 0 {
		return nil, errWrapf("%w: 答题记录未绑定试卷版本", ErrNotFound)
	}
	items, err := s.paperRepo.ListPaperVersionQuestions(ctx, attempt.PaperVersionID)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// orderedVersionQuestions loads the bound version and its frozen questions in
// the per-student shuffled order persisted on the attempt.
func (s *AttemptService) orderedVersionQuestions(ctx context.Context, attempt *model.ExamAttempt) (*model.PaperVersion, []model.PaperVersionQuestion, error) {
	if attempt.PaperVersionID == 0 {
		return nil, nil, errWrapf("%w: 答题记录未绑定试卷版本", ErrNotFound)
	}
	version, err := s.paperRepo.FindPaperVersionByID(ctx, attempt.PaperVersionID)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.paperRepo.ListPaperVersionQuestions(ctx, attempt.PaperVersionID)
	if err != nil {
		return nil, nil, err
	}
	order := parseOrder(attempt.QuestionOrder)
	items = orderVersionQuestions(items, order)
	return version, items, nil
}

// findVersionQuestion returns one frozen question by its version-question id.
func (s *AttemptService) findVersionQuestion(ctx context.Context, attempt *model.ExamAttempt, versionQuestionID uint) (model.PaperVersionQuestion, bool, error) {
	items, err := s.versionQuestions(ctx, attempt)
	if err != nil {
		return model.PaperVersionQuestion{}, false, err
	}
	for _, it := range items {
		if it.ID == versionQuestionID {
			return it, true, nil
		}
	}
	return model.PaperVersionQuestion{}, false, nil
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

func orderVersionQuestions(items []model.PaperVersionQuestion, order []uint) []model.PaperVersionQuestion {
	if len(order) == 0 {
		return items
	}
	byID := make(map[uint]model.PaperVersionQuestion, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	result := make([]model.PaperVersionQuestion, 0, len(items))
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
