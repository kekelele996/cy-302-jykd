package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// fakeDB is an in-memory implementation of all repository contracts used by the
// exam/attempt services. Transaction-taking methods ignore the *gorm.DB handle
// (nil in tests) and apply changes immediately, which preserves the ordering
// semantics the services rely on.
type fakeDB struct {
	exams            map[uint]*model.Exam
	questions        map[uint]*model.Question
	examQuestions    map[uint]*model.ExamQuestion
	versions         map[uint]*model.PaperVersion
	versionQuestions map[uint]*model.PaperVersionQuestion
	attempts         map[uint]*model.ExamAttempt
	answers          map[uint]*model.Answer
	wrongs           map[uint]*model.WrongQuestion
	nextExamID       uint
	nextQuestionID   uint
	nextEQID         uint
	nextVersionID    uint
	nextVQID         uint
	nextAttemptID    uint
	nextAnswerID     uint
	nextWrongID      uint
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		exams:            map[uint]*model.Exam{},
		questions:        map[uint]*model.Question{},
		examQuestions:    map[uint]*model.ExamQuestion{},
		versions:         map[uint]*model.PaperVersion{},
		versionQuestions: map[uint]*model.PaperVersionQuestion{},
		attempts:         map[uint]*model.ExamAttempt{},
		answers:          map[uint]*model.Answer{},
		wrongs:           map[uint]*model.WrongQuestion{},
		nextExamID:       1,
		nextQuestionID:   1,
		nextEQID:         1,
		nextVersionID:    1,
		nextVQID:         1,
		nextAttemptID:    1,
		nextAnswerID:     1,
		nextWrongID:      1,
	}
}

func (f *fakeDB) seedQuestion(t *testing.T, qtype, content, answer string) *model.Question {
	t.Helper()
	options := "[]"
	if qtype == constants.QuestionSingle {
		options = mustMarshalJSON(t, []dto.Option{{Key: "A", Text: "选项A"}, {Key: "B", Text: "选项B"}})
	}
	q := &model.Question{
		ID:             f.nextQuestionID,
		Type:           qtype,
		Content:        content,
		Options:        options,
		Answer:         answer,
		Analysis:       "解析-" + content,
		Difficulty:     constants.DifficultyEasy,
		KnowledgePoint: "kp",
		Score:          2,
	}
	f.nextQuestionID++
	f.questions[q.ID] = q
	return q
}

// ---- QuestionRepo ----

func (f *fakeDB) CreateQuestion(_ context.Context, q *model.Question) error {
	q.ID = f.nextQuestionID
	f.nextQuestionID++
	f.questions[q.ID] = q
	return nil
}
func (f *fakeDB) CreateQuestionsBatch(_ context.Context, qs []model.Question) error {
	for i := range qs {
		qs[i].ID = f.nextQuestionID
		f.nextQuestionID++
		f.questions[qs[i].ID] = &qs[i]
	}
	return nil
}
func (f *fakeDB) FindQuestionByID(_ context.Context, id uint) (*model.Question, error) {
	q, ok := f.questions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *q
	return &cp, nil
}
func (f *fakeDB) UpdateQuestion(_ context.Context, q *model.Question) error {
	if _, ok := f.questions[q.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *q
	f.questions[q.ID] = &cp
	return nil
}
func (f *fakeDB) DeleteQuestion(_ context.Context, id uint) error {
	if _, ok := f.questions[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.questions, id)
	return nil
}
func (f *fakeDB) ListQuestions(_ context.Context, _ repository.QuestionFilter, page, pageSize int) ([]model.Question, int64, error) {
	ids := sortedQuestionIDs(f.questions)
	out := make([]model.Question, 0, len(ids))
	for _, id := range ids {
		out = append(out, *f.questions[id])
	}
	return out, int64(len(out)), nil
}
func (f *fakeDB) ListQuestionsByTypeDifficulty(_ context.Context, qtype, difficulty string) ([]model.Question, error) {
	out := make([]model.Question, 0)
	for _, id := range sortedQuestionIDs(f.questions) {
		q := f.questions[id]
		if q.Type != qtype {
			continue
		}
		if difficulty != "" && q.Difficulty != difficulty {
			continue
		}
		out = append(out, *q)
	}
	return out, nil
}
func (f *fakeDB) FindQuestionsByIDs(_ context.Context, ids []uint) (map[uint]model.Question, error) {
	return f.findQuestionsByIDs(ids), nil
}
func (f *fakeDB) FindQuestionsByIDsTx(_ context.Context, _ *gorm.DB, ids []uint) (map[uint]model.Question, error) {
	return f.findQuestionsByIDs(ids), nil
}
func (f *fakeDB) findQuestionsByIDs(ids []uint) map[uint]model.Question {
	out := make(map[uint]model.Question, len(ids))
	for _, id := range ids {
		if q, ok := f.questions[id]; ok {
			out[id] = *q
		}
	}
	return out
}

// ---- ExamRepo ----

func (f *fakeDB) CreateExam(_ context.Context, exam *model.Exam) error {
	exam.ID = f.nextExamID
	f.nextExamID++
	cp := *exam
	f.exams[exam.ID] = &cp
	return nil
}
func (f *fakeDB) FindExamByID(_ context.Context, id uint) (*model.Exam, error) {
	e, ok := f.exams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *e
	return &cp, nil
}
func (f *fakeDB) UpdateExam(_ context.Context, exam *model.Exam) error {
	if _, ok := f.exams[exam.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *exam
	f.exams[exam.ID] = &cp
	return nil
}
func (f *fakeDB) DeleteExam(_ context.Context, id uint) error {
	if _, ok := f.exams[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.exams, id)
	for eqID, eq := range f.examQuestions {
		if eq.ExamID == id {
			delete(f.examQuestions, eqID)
		}
	}
	return nil
}
func (f *fakeDB) ListExams(_ context.Context, _ repository.ExamFilter, page, pageSize int) ([]model.Exam, int64, error) {
	ids := make([]uint, 0, len(f.exams))
	for id := range f.exams {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	out := make([]model.Exam, 0, len(ids))
	for _, id := range ids {
		out = append(out, *f.exams[id])
	}
	return out, int64(len(out)), nil
}
func (f *fakeDB) ReplaceExamQuestions(_ context.Context, examID uint, items []model.ExamQuestion) error {
	return f.replaceExamQuestions(examID, items)
}
func (f *fakeDB) ReplaceExamQuestionsTx(_ context.Context, _ *gorm.DB, examID uint, items []model.ExamQuestion) error {
	return f.replaceExamQuestions(examID, items)
}
func (f *fakeDB) replaceExamQuestions(examID uint, items []model.ExamQuestion) error {
	for id, eq := range f.examQuestions {
		if eq.ExamID == examID {
			delete(f.examQuestions, id)
		}
	}
	for i := range items {
		items[i].ID = f.nextEQID
		items[i].ExamID = examID
		f.nextEQID++
		cp := items[i]
		f.examQuestions[cp.ID] = &cp
	}
	return nil
}
func (f *fakeDB) UpdateExamTotalScoreTx(_ context.Context, _ *gorm.DB, examID uint, total float64) error {
	e, ok := f.exams[examID]
	if !ok {
		return repository.ErrNotFound
	}
	e.TotalScore = total
	return nil
}
func (f *fakeDB) ListExamQuestions(_ context.Context, examID uint) ([]model.ExamQuestion, error) {
	return f.listExamQuestions(examID), nil
}
func (f *fakeDB) ListExamQuestionsTx(_ context.Context, _ *gorm.DB, examID uint) ([]model.ExamQuestion, error) {
	return f.listExamQuestions(examID), nil
}
func (f *fakeDB) listExamQuestions(examID uint) []model.ExamQuestion {
	out := make([]model.ExamQuestion, 0)
	for _, eq := range f.examQuestions {
		if eq.ExamID == examID {
			out = append(out, *eq)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}
func (f *fakeDB) CountExamQuestions(_ context.Context, examID uint) (int64, error) {
	return int64(len(f.listExamQuestions(examID))), nil
}
func (f *fakeDB) CountExamQuestionsTx(_ context.Context, _ *gorm.DB, examID uint) (int64, error) {
	return int64(len(f.listExamQuestions(examID))), nil
}
func (f *fakeDB) PublishExamIfDraft(_ context.Context, _ *gorm.DB, id, versionID uint) (int64, error) {
	e, ok := f.exams[id]
	if !ok || e.Status != constants.ExamDraft {
		return 0, nil
	}
	e.Status = constants.ExamPublished
	e.CurrentVersionID = versionID
	e.Revision++
	return 1, nil
}

// ---- PaperRepo ----

func (f *fakeDB) WithTransaction(_ context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}
func (f *fakeDB) CreatePaperVersion(_ context.Context, _ *gorm.DB, in repository.FreezePaperInput) (*model.PaperVersion, error) {
	total := 0.0
	for _, it := range in.Items {
		total += it.Score
	}
	v := &model.PaperVersion{
		ID:         f.nextVersionID,
		ExamID:     in.ExamID,
		VersionNo:  in.VersionNo,
		Title:      in.Title,
		TotalScore: total,
		SnapshotAt: time.Now(),
		CreatedBy:  in.CreatedBy,
	}
	f.nextVersionID++
	f.versions[v.ID] = v
	for _, it := range in.Items {
		fq := &model.PaperVersionQuestion{
			ID:             f.nextVQID,
			VersionID:      v.ID,
			QuestionID:     it.QuestionID,
			Type:           it.Type,
			Content:        it.Content,
			Options:        it.Options,
			Answer:         it.Answer,
			Analysis:       it.Analysis,
			Difficulty:     it.Difficulty,
			KnowledgePoint: it.KnowledgePoint,
			Score:          it.Score,
			SortOrder:      it.SortOrder,
		}
		f.nextVQID++
		f.versionQuestions[fq.ID] = fq
	}
	cp := *v
	return &cp, nil
}
func (f *fakeDB) FindPaperVersionByID(_ context.Context, id uint) (*model.PaperVersion, error) {
	v, ok := f.versions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *v
	return &cp, nil
}
func (f *fakeDB) FindCurrentPaperVersion(_ context.Context, examID uint) (*model.PaperVersion, error) {
	e, ok := f.exams[examID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if e.CurrentVersionID == 0 {
		return nil, repository.ErrNotFound
	}
	return f.FindPaperVersionByID(context.Background(), e.CurrentVersionID)
}
func (f *fakeDB) ListPaperVersions(_ context.Context, examID uint) ([]model.PaperVersion, error) {
	out := make([]model.PaperVersion, 0)
	for _, v := range f.versions {
		if v.ExamID == examID {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionNo > out[j].VersionNo })
	return out, nil
}
func (f *fakeDB) MaxPaperVersionNo(_ context.Context, _ *gorm.DB, examID uint) (int, error) {
	maxNo := 0
	for _, v := range f.versions {
		if v.ExamID == examID && v.VersionNo > maxNo {
			maxNo = v.VersionNo
		}
	}
	return maxNo, nil
}
func (f *fakeDB) ListPaperVersionQuestions(_ context.Context, versionID uint) ([]model.PaperVersionQuestion, error) {
	out := make([]model.PaperVersionQuestion, 0)
	for _, fq := range f.versionQuestions {
		if fq.VersionID == versionID {
			out = append(out, *fq)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}
func (f *fakeDB) LockExamForUpdate(_ context.Context, _ *gorm.DB, id uint) (*model.Exam, error) {
	return f.FindExamByID(context.Background(), id)
}
func (f *fakeDB) PointExamToVersion(_ context.Context, _ *gorm.DB, examID, versionID uint, status string) error {
	e, ok := f.exams[examID]
	if !ok {
		return repository.ErrNotFound
	}
	e.CurrentVersionID = versionID
	e.Status = status
	e.Revision++
	return nil
}

// ---- AttemptRepo ----

func (f *fakeDB) CreateAttempt(_ context.Context, a *model.ExamAttempt) error {
	a.ID = f.nextAttemptID
	f.nextAttemptID++
	cp := *a
	f.attempts[a.ID] = &cp
	return nil
}
func (f *fakeDB) FindAttemptByID(_ context.Context, id uint) (*model.ExamAttempt, error) {
	a, ok := f.attempts[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *a
	return &cp, nil
}
func (f *fakeDB) UpdateAttempt(_ context.Context, a *model.ExamAttempt) error {
	existing, ok := f.attempts[a.ID]
	if !ok {
		return repository.ErrNotFound
	}
	existing.Status = a.Status
	existing.SubmittedAt = a.SubmittedAt
	existing.Deadline = a.Deadline
	existing.ObjectiveScore = a.ObjectiveScore
	existing.TotalScore = a.TotalScore
	return nil
}
func (f *fakeDB) FindInProgressAttempt(_ context.Context, examID, studentID uint) (*model.ExamAttempt, error) {
	for _, a := range f.attempts {
		if a.ExamID == examID && a.StudentID == studentID && a.Status == constants.AttemptInProgress {
			cp := *a
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeDB) ListAttemptsByStudent(_ context.Context, studentID, examID uint, page, pageSize int) ([]model.ExamAttempt, int64, error) {
	out := make([]model.ExamAttempt, 0)
	for _, a := range f.attempts {
		if a.StudentID != studentID {
			continue
		}
		if examID != 0 && a.ExamID != examID {
			continue
		}
		out = append(out, *a)
	}
	return out, int64(len(out)), nil
}
func (f *fakeDB) ListAttemptsByExam(_ context.Context, examID uint) ([]model.ExamAttempt, error) {
	out := make([]model.ExamAttempt, 0)
	for _, a := range f.attempts {
		if a.ExamID == examID {
			out = append(out, *a)
		}
	}
	return out, nil
}

// ---- AnswerRepo ----

func (f *fakeDB) SaveAnswer(_ context.Context, answer *model.Answer) error {
	for _, existing := range f.answers {
		if existing.AttemptID == answer.AttemptID && existing.ExamQuestionID == answer.ExamQuestionID {
			existing.AnswerText = answer.AnswerText
			existing.IsCorrect = answer.IsCorrect
			existing.Score = answer.Score
			existing.Marked = answer.Marked
			existing.GradedBy = answer.GradedBy
			return nil
		}
	}
	answer.ID = f.nextAnswerID
	f.nextAnswerID++
	cp := *answer
	f.answers[answer.ID] = &cp
	return nil
}
func (f *fakeDB) ListAnswersByAttempt(_ context.Context, attemptID uint) ([]model.Answer, error) {
	out := make([]model.Answer, 0)
	for _, a := range f.answers {
		if a.AttemptID == attemptID {
			out = append(out, *a)
		}
	}
	return out, nil
}

// ---- WrongRepo ----

func (f *fakeDB) UpsertWrongQuestion(_ context.Context, w *model.WrongQuestion) error {
	for _, existing := range f.wrongs {
		if existing.StudentID == w.StudentID && existing.QuestionID == w.QuestionID {
			existing.WrongCount++
			existing.LastWrongAt = w.LastWrongAt
			return nil
		}
	}
	w.ID = f.nextWrongID
	f.nextWrongID++
	f.wrongs[w.ID] = w
	return nil
}
func (f *fakeDB) ListWrongQuestions(_ context.Context, studentID uint, kp string, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	return nil, 0, nil
}
func (f *fakeDB) DeleteWrongQuestion(_ context.Context, id, studentID uint) error { return nil }
func (f *fakeDB) MarkWrongQuestionResolved(_ context.Context, id, studentID uint) error {
	return nil
}

func sortedQuestionIDs(m map[uint]*model.Question) []uint {
	ids := make([]uint, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func mustMarshalJSON(t *testing.T, v any) string {
	t.Helper()
	s, err := marshalOptions(v.([]dto.Option))
	if err != nil {
		t.Fatalf("marshal options: %v", err)
	}
	return s
}

func newExamServices(f *fakeDB) (*ExamService, *AttemptService) {
	logger := slog.New(slog.NewTextHandler(&testLogWriter{}, nil))
	examSvc := NewExamService(f, f, f, logger)
	attemptSvc := NewAttemptService(f, f, f, f, f, f, logger)
	return examSvc, attemptSvc
}

type testLogWriter struct{}

func (testLogWriter) Write(p []byte) (int, error) { return len(p), nil }

func createSingleChoiceExam(t *testing.T, ctx context.Context, svc *ExamService, teacherID uint) *dto.ExamResponse {
	t.Helper()
	resp, err := svc.Create(ctx, teacherID, dto.ExamCreateRequest{
		Title:           "版本冻结测试",
		DurationMinutes: 60,
		QuestionConfig: []dto.PaperQuestionConfig{
			{Type: constants.QuestionSingle, Count: 1, Score: 5, Difficulty: constants.DifficultyEasy},
		},
	})
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}
	return resp
}

func TestPublishFreezesQuestionContent(t *testing.T) {
	ctx := context.Background()
	f := newFakeDB()
	q := f.seedQuestion(t, constants.QuestionSingle, "原始题干", mustAnswerJSON(t, "A"))
	examSvc, attemptSvc := newExamServices(f)
	exam := createSingleChoiceExam(t, ctx, examSvc, 100)

	if _, err := examSvc.Publish(ctx, constants.RoleTeacher, 100, exam.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Student starts against v1 and saves the correct answer.
	started, err := attemptSvc.Start(ctx, 1, exam.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.VersionNo != 1 || started.PaperVersionID == 0 {
		t.Fatalf("attempt should bind to v1, got version_no=%d version_id=%d", started.VersionNo, started.PaperVersionID)
	}
	if started.Questions[0].Content != "原始题干" {
		t.Fatalf("frozen content mismatch: %q", started.Questions[0].Content)
	}
	eqID := started.Questions[0].ExamQuestionID
	if err := attemptSvc.SaveAnswer(ctx, 1, started.AttemptID, dto.AnswerSubmitRequest{
		ExamQuestionID: eqID, Answer: "A",
	}); err != nil {
		t.Fatalf("save answer: %v", err)
	}

	// Teacher edits the bank question (stem + reference answer) after publish.
	q.Content = "修改后的题干"
	q.Answer = mustAnswerJSON(t, "B")
	if err := f.UpdateQuestion(ctx, q); err != nil {
		t.Fatalf("update question: %v", err)
	}

	// Refreshing the in-progress exam keeps the frozen stem.
	resumed, err := attemptSvc.Current(ctx, 1, exam.ID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.Questions[0].Content != "原始题干" {
		t.Fatalf("refreshed exam must show frozen content, got %q", resumed.Questions[0].Content)
	}

	// Submitting grades against the frozen answer A => full score.
	if err := attemptSvc.Submit(ctx, 1, started.AttemptID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	submitted, _ := f.FindAttemptByID(ctx, started.AttemptID)
	if submitted.TotalScore != 5 {
		t.Fatalf("objective grading must use frozen answer, score=%v", submitted.TotalScore)
	}

	// Historical review and report keep v1 content.
	detail, err := attemptSvc.Detail(ctx, constants.RoleStudent, 1, started.AttemptID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.VersionNo != 1 || detail.Questions[0].Content != "原始题干" {
		t.Fatalf("historical detail must read v1 frozen content")
	}
	if correct, _ := detail.Questions[0].CorrectAnswer.(string); correct != "A" {
		t.Fatalf("historical standard answer must be frozen A, got %v", detail.Questions[0].CorrectAnswer)
	}
	report, err := attemptSvc.Report(ctx, constants.RoleStudent, 1, started.AttemptID)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.VersionNo != 1 {
		t.Fatalf("report must reference v1")
	}

	// Staff paper review of v1 is also frozen.
	paper, err := examSvc.ListPaperQuestions(ctx, constants.RoleTeacher, 100, exam.ID, 1)
	if err != nil {
		t.Fatalf("list v1 paper: %v", err)
	}
	if paper[0].Question.Content != "原始题干" {
		t.Fatalf("staff review of v1 must be frozen")
	}
}

func TestRepeatPublishIsIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newFakeDB()
	f.seedQuestion(t, constants.QuestionSingle, "题", mustAnswerJSON(t, "A"))
	examSvc, _ := newExamServices(f)
	exam := createSingleChoiceExam(t, ctx, examSvc, 100)

	first, err := examSvc.Publish(ctx, constants.RoleTeacher, 100, exam.ID)
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}
	second, err := examSvc.Publish(ctx, constants.RoleTeacher, 100, exam.ID)
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate publish must return the same version, got %d vs %d", first.ID, second.ID)
	}
	versions, _ := f.ListPaperVersions(ctx, exam.ID)
	if len(versions) != 1 {
		t.Fatalf("duplicate publish created %d versions, want 1", len(versions))
	}
}

func TestRegroupCreatesNewVersionAndKeepsOldReadable(t *testing.T) {
	ctx := context.Background()
	f := newFakeDB()
	f.seedQuestion(t, constants.QuestionSingle, "题1", mustAnswerJSON(t, "A"))
	f.seedQuestion(t, constants.QuestionSingle, "题2", mustAnswerJSON(t, "A"))
	examSvc, attemptSvc := newExamServices(f)
	exam := createSingleChoiceExam(t, ctx, examSvc, 100)

	if _, err := examSvc.Publish(ctx, constants.RoleTeacher, 100, exam.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Student 1 starts while v1 is current.
	oldAttempt, err := attemptSvc.Start(ctx, 1, exam.ID)
	if err != nil {
		t.Fatalf("start old: %v", err)
	}

	// Teacher regroups; revision after publish is 1.
	current, err := examSvc.Get(ctx, constants.RoleTeacher, 100, exam.ID)
	if err != nil {
		t.Fatalf("get exam: %v", err)
	}
	regroup, err := examSvc.Regroup(ctx, constants.RoleTeacher, 100, exam.ID, dto.RegroupRequest{
		ExpectedRevision: current.Revision,
		QuestionConfig: []dto.PaperQuestionConfig{
			{Type: constants.QuestionSingle, Count: 1, Score: 8, Difficulty: constants.DifficultyEasy},
		},
	})
	if err != nil {
		t.Fatalf("regroup: %v", err)
	}
	if regroup.VersionNo != 2 {
		t.Fatalf("regroup must create v2, got v%d", regroup.VersionNo)
	}

	versions, err := examSvc.ListVersions(ctx, constants.RoleTeacher, 100, exam.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("expected 2 retained versions, got %d err=%v", len(versions), err)
	}
	if versions[0].VersionNo != 2 || !versions[0].IsCurrent || versions[1].VersionNo != 1 || versions[1].IsCurrent {
		t.Fatalf("version list order/current flags wrong: %+v", versions)
	}

	// In-progress student 1 is still bound to v1, even after refresh.
	resumed, err := attemptSvc.Current(ctx, 1, exam.ID)
	if err != nil {
		t.Fatalf("resume old: %v", err)
	}
	if resumed.PaperVersionID != oldAttempt.PaperVersionID || resumed.VersionNo != 1 {
		t.Fatalf("in-progress attempt must stay on v1, got v%d", resumed.VersionNo)
	}

	// A new student gets v2 with the new per-question score.
	newAttempt, err := attemptSvc.Start(ctx, 2, exam.ID)
	if err != nil {
		t.Fatalf("start new: %v", err)
	}
	if newAttempt.VersionNo != 2 || newAttempt.Questions[0].Score != 8 {
		t.Fatalf("new attempt must use v2 with score 8, got v%d score=%v", newAttempt.VersionNo, newAttempt.Questions[0].Score)
	}

	// Stale concurrent regroup loses: only one simultaneous change takes effect.
	_, err = examSvc.Regroup(ctx, constants.RoleTeacher, 100, exam.ID, dto.RegroupRequest{
		ExpectedRevision: current.Revision, // still 1, but revision is now 2
		QuestionConfig: []dto.PaperQuestionConfig{
			{Type: constants.QuestionSingle, Count: 1, Score: 3, Difficulty: constants.DifficultyEasy},
		},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale regroup must conflict, got err=%v", err)
	}
	versionsAfter, _ := examSvc.ListVersions(ctx, constants.RoleTeacher, 100, exam.ID)
	if len(versionsAfter) != 2 {
		t.Fatalf("rejected regroup must not create a version, got %d", len(versionsAfter))
	}
}

func TestWithdrawnQuestionKeepsHistoricalPaper(t *testing.T) {
	ctx := context.Background()
	f := newFakeDB()
	q := f.seedQuestion(t, constants.QuestionSingle, "将要被撤回的题", mustAnswerJSON(t, "A"))
	examSvc, attemptSvc := newExamServices(f)
	exam := createSingleChoiceExam(t, ctx, examSvc, 100)
	if _, err := examSvc.Publish(ctx, constants.RoleTeacher, 100, exam.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	started, err := attemptSvc.Start(ctx, 1, exam.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	eqID := started.Questions[0].ExamQuestionID
	if err := attemptSvc.SaveAnswer(ctx, 1, started.AttemptID, dto.AnswerSubmitRequest{
		ExamQuestionID: eqID, Answer: "A",
	}); err != nil {
		t.Fatalf("save answer: %v", err)
	}
	if err := attemptSvc.Submit(ctx, 1, started.AttemptID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Withdraw the source question from the bank.
	if err := f.DeleteQuestion(ctx, q.ID); err != nil {
		t.Fatalf("delete question: %v", err)
	}

	detail, err := attemptSvc.Detail(ctx, constants.RoleStudent, 1, started.AttemptID)
	if err != nil {
		t.Fatalf("detail after withdrawal: %v", err)
	}
	if detail.Questions[0].Content != "将要被撤回的题" {
		t.Fatalf("withdrawn question content must remain frozen in history")
	}
	if detail.TotalScore != 5 {
		t.Fatalf("historical score must survive question withdrawal, got %v", detail.TotalScore)
	}
	paper, err := examSvc.ListPaperQuestions(ctx, constants.RoleAdmin, 0, exam.ID, 1)
	if err != nil {
		t.Fatalf("review v1 after withdrawal: %v", err)
	}
	if len(paper) != 1 || paper[0].Question.Content != "将要被撤回的题" {
		t.Fatalf("frozen v1 must still be fully reviewable after withdrawal")
	}
}

func mustAnswerJSON(t *testing.T, v any) string {
	t.Helper()
	s, err := marshalAnswer(v)
	if err != nil {
		t.Fatalf("marshal answer: %v", err)
	}
	return s
}
