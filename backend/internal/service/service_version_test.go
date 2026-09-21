package service

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// In-memory fakes used to verify version-freeze business rules without MySQL.

type fakeExamRepo struct {
	mu    sync.Mutex
	exams map[uint]*model.Exam
}

func newFakeExamRepo() *fakeExamRepo {
	return &fakeExamRepo{exams: map[uint]*model.Exam{}}
}

func (r *fakeExamRepo) CreateExam(_ context.Context, e *model.Exam) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.ID == 0 {
		e.ID = uint(len(r.exams) + 1)
	}
	cp := *e
	r.exams[e.ID] = &cp
	return nil
}

func (r *fakeExamRepo) FindExamByID(_ context.Context, id uint) (*model.Exam, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.exams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (r *fakeExamRepo) UpdateExam(_ context.Context, e *model.Exam) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.exams[e.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *e
	r.exams[e.ID] = &cp
	return nil
}

func (r *fakeExamRepo) DeleteExam(_ context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.exams[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.exams, id)
	return nil
}

func (r *fakeExamRepo) ListExams(_ context.Context, _ repository.ExamFilter, _, _ int) ([]model.Exam, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.Exam, 0, len(r.exams))
	for _, e := range r.exams {
		out = append(out, *e)
	}
	return out, int64(len(out)), nil
}

type fakeVersionRepo struct {
	mu       sync.Mutex
	versions map[uint]*model.ExamVersion
	snaps    map[uint][]model.ExamVersionQuestion
	examsRef map[uint]*model.Exam
}

func newFakeVersionRepo() *fakeVersionRepo {
	return &fakeVersionRepo{versions: map[uint]*model.ExamVersion{}, snaps: map[uint][]model.ExamVersionQuestion{}}
}

func (r *fakeVersionRepo) CreateVersion(_ context.Context, v *model.ExamVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v.ID == 0 {
		v.ID = uint(len(r.versions) + 1)
	}
	cp := *v
	r.versions[v.ID] = &cp
	return nil
}

func (r *fakeVersionRepo) CreateExamWithDraftVersion(_ context.Context, exam *model.Exam, compose repository.DraftComposer) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if exam.ID == 0 {
		exam.ID = uint(len(r.examsRef) + 1)
	}
	cpExam := *exam
	r.examsRef[exam.ID] = &cpExam
	items, total, duration, err := compose()
	if err != nil {
		return nil, err
	}
	version := &model.ExamVersion{
		ID:              uint(len(r.versions) + 1),
		ExamID:          exam.ID,
		VersionNo:       1,
		Status:          constants.VersionDraft,
		TotalScore:      total,
		DurationMinutes: duration,
		CreatedBy:       exam.CreatedBy,
		CreatedAt:       time.Now(),
	}
	for i := range items {
		items[i].VersionID = version.ID
		items[i].ExamID = exam.ID
	}
	r.versions[version.ID] = version
	r.snaps[version.ID] = items
	cp := *version
	return &cp, nil
}

func (r *fakeVersionRepo) CreateVersionQuestions(_ context.Context, items []model.ExamVersionQuestion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snaps[items[0].VersionID] = append([]model.ExamVersionQuestion(nil), items...)
	return nil
}

func (r *fakeVersionRepo) FindVersionByID(_ context.Context, id uint) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *fakeVersionRepo) FindVersionByExamAndNo(_ context.Context, examID uint, no int) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versions {
		if v.ExamID == examID && v.VersionNo == no {
			cp := *v
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *fakeVersionRepo) FindDraftVersion(_ context.Context, examID uint) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *model.ExamVersion
	for _, v := range r.versions {
		if v.ExamID == examID && v.Status == constants.VersionDraft && (best == nil || v.VersionNo > best.VersionNo) {
			best = v
		}
	}
	if best == nil {
		return nil, repository.ErrNotFound
	}
	cp := *best
	return &cp, nil
}

func (r *fakeVersionRepo) FindCurrentVersion(_ context.Context, examID uint) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *model.ExamVersion
	for _, v := range r.versions {
		if v.ExamID == examID && v.Status == constants.VersionPublished && (best == nil || v.VersionNo > best.VersionNo) {
			best = v
		}
	}
	if best == nil {
		return nil, repository.ErrNotFound
	}
	cp := *best
	return &cp, nil
}

func (r *fakeVersionRepo) ListVersions(_ context.Context, examID uint) ([]model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ExamVersion, 0)
	for _, v := range r.versions {
		if v.ExamID == examID {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (r *fakeVersionRepo) ListVersionQuestions(_ context.Context, versionID uint) ([]model.ExamVersionQuestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.ExamVersionQuestion(nil), r.snaps[versionID]...), nil
}

func (r *fakeVersionRepo) CountVersionQuestions(_ context.Context, versionID uint) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.snaps[versionID])), nil
}

func (r *fakeVersionRepo) DeleteVersions(_ context.Context, examID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, v := range r.versions {
		if v.ExamID == examID {
			delete(r.snaps, id)
			delete(r.versions, id)
		}
	}
	return nil
}

// SaveDraftVersion mirrors the real transactional semantics.
func (r *fakeVersionRepo) SaveDraftVersion(_ context.Context, examID uint, compose repository.DraftComposer) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	exam := r.examsRef[examID]
	var draft *model.ExamVersion
	for _, v := range r.versions {
		if v.ExamID == examID && v.Status == constants.VersionDraft && (draft == nil || v.VersionNo > draft.VersionNo) {
			draft = v
		}
	}
	if exam.Status != constants.ExamDraft && draft != nil {
		return nil, repository.ErrConflict
	}
	items, total, duration, err := compose()
	if err != nil {
		return nil, err
	}
	if draft != nil {
		draft.TotalScore = total
		draft.DurationMinutes = duration
		r.snaps[draft.ID] = items
		cp := *draft
		return &cp, nil
	}
	no := 1
	for _, v := range r.versions {
		if v.ExamID == examID && v.VersionNo >= no {
			no = v.VersionNo + 1
		}
	}
	v := &model.ExamVersion{
		ID:              uint(len(r.versions) + 1),
		ExamID:          examID,
		VersionNo:       no,
		Status:          constants.VersionDraft,
		TotalScore:      total,
		DurationMinutes: duration,
		CreatedBy:       exam.CreatedBy,
		CreatedAt:       time.Now(),
	}
	r.versions[v.ID] = v
	r.snaps[v.ID] = items
	cp := *v
	return &cp, nil
}

// PublishVersion mirrors publish atomicity and idempotency rules.
func (r *fakeVersionRepo) PublishVersion(_ context.Context, examID uint) (*model.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	exam := r.examsRef[examID]
	var draft *model.ExamVersion
	for _, v := range r.versions {
		if v.ExamID == examID && v.Status == constants.VersionDraft && (draft == nil || v.VersionNo > draft.VersionNo) {
			draft = v
		}
	}
	// Repeated publish: no pending draft means it already took effect once.
	if draft == nil {
		if exam.Status == constants.ExamPublished || exam.Status == constants.ExamClosed {
			return nil, repository.ErrConflict
		}
		return nil, repository.ErrValidation
	}
	if len(r.snaps[draft.ID]) == 0 {
		return nil, repository.ErrValidation
	}
	for _, v := range r.versions {
		if v.ExamID == examID && v.Status == constants.VersionPublished {
			v.Status = constants.VersionArchived
		}
	}
	now := time.Now()
	draft.Status = constants.VersionPublished
	draft.PublishedAt = &now
	exam.Status = constants.ExamPublished
	exam.CurrentVersionID = draft.ID
	exam.TotalScore = draft.TotalScore
	exam.DurationMinutes = draft.DurationMinutes
	cp := *draft
	return &cp, nil
}

func (r *fakeVersionRepo) DiscardDraftVersion(_ context.Context, examID, versionID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[versionID]
	if !ok || v.ExamID != examID || v.Status != constants.VersionDraft {
		return repository.ErrNotFound
	}
	delete(r.versions, versionID)
	delete(r.snaps, versionID)
	return nil
}

type fakeQuestionRepo struct {
	questions map[uint]model.Question
}

func newFakeQuestionRepo() *fakeQuestionRepo {
	return &fakeQuestionRepo{questions: map[uint]model.Question{}}
}

func (r *fakeQuestionRepo) CreateQuestion(_ context.Context, q *model.Question) error {
	r.questions[q.ID] = *q
	return nil
}
func (r *fakeQuestionRepo) CreateQuestionsBatch(_ context.Context, _ []model.Question) error { return nil }
func (r *fakeQuestionRepo) FindQuestionByID(_ context.Context, id uint) (*model.Question, error) {
	q, ok := r.questions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &q, nil
}
func (r *fakeQuestionRepo) UpdateQuestion(_ context.Context, q *model.Question) error {
	r.questions[q.ID] = *q
	return nil
}
func (r *fakeQuestionRepo) DeleteQuestion(_ context.Context, _ uint) error { return nil }
func (r *fakeQuestionRepo) ListQuestions(_ context.Context, _ repository.QuestionFilter, _, _ int) ([]model.Question, int64, error) {
	return nil, 0, nil
}
func (r *fakeQuestionRepo) ListQuestionsByTypeDifficulty(_ context.Context, qtype, difficulty string) ([]model.Question, error) {
	out := make([]model.Question, 0)
	for _, q := range r.questions {
		if q.Type == qtype && (difficulty == "" || q.Difficulty == difficulty) {
			out = append(out, q)
		}
	}
	return out, nil
}
func (r *fakeQuestionRepo) FindQuestionsByIDs(_ context.Context, ids []uint) (map[uint]model.Question, error) {
	out := map[uint]model.Question{}
	for _, id := range ids {
		if q, ok := r.questions[id]; ok {
			out[id] = q
		}
	}
	return out, nil
}

func newExamServiceForTest() (*ExamService, *fakeExamRepo, *fakeVersionRepo, *fakeQuestionRepo) {
	exams := newFakeExamRepo()
	versions := newFakeVersionRepo()
	questions := newFakeQuestionRepo()
	versions.examsRef = exams.exams
	svc := NewExamService(exams, versions, questions, testLogger())
	return svc, exams, versions, questions
}

func seedQuestions(repo *fakeQuestionRepo, n int, qtype, difficulty string) {
	for i := 0; i < n; i++ {
		id := uint(i + 1)
		if qtype == constants.QuestionSingle {
			id = uint(i + 1)
		}
		repo.questions[id] = model.Question{
			ID:             id,
			Type:           qtype,
			Content:        "题干 v0",
			Options:        `[{"key":"A","text":"原选项A"},{"key":"B","text":"原选项B"}]`,
			Answer:         `"A"`,
			Analysis:       "原解析",
			Difficulty:     difficulty,
			KnowledgePoint: "kp",
			Score:          2,
		}
	}
}

func regenRequest() dto.PaperRegenerateRequest {
	return dto.PaperRegenerateRequest{
		DurationMinutes: 30,
		QuestionConfig: []dto.PaperQuestionConfig{
			{Type: constants.QuestionSingle, Count: 3, Score: 2, Difficulty: "easy"},
		},
	}
}

func createReq() dto.ExamCreateRequest {
	r := regenRequest()
	return dto.ExamCreateRequest{
		Title:           "期末考试",
		DurationMinutes: r.DurationMinutes,
		QuestionConfig:  r.QuestionConfig,
	}
}

// TestPublishFreezesVersion verifies that after publish, editing or
// withdrawing the source question cannot change the frozen paper.
func TestPublishFreezesVersion(t *testing.T) {
	svc, _, versions, questions := newExamServiceForTest()
	seedQuestions(questions, 3, constants.QuestionSingle, "easy")
	ctx := context.Background()

	created, err := svc.Create(ctx, 1, createReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Publish(ctx, constants.RoleAdmin, 0, created.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Mutate and withdraw source questions after publication.
	for id := uint(1); id <= 3; id++ {
		q := questions.questions[id]
		q.Content = "题干被篡改"
		q.Options = `[{"key":"A","text":"被改选项"}]`
		q.Answer = `"B"`
		questions.questions[id] = q
	}

	current, err := versions.FindCurrentVersion(ctx, created.ID)
	if err != nil {
		t.Fatalf("current version: %v", err)
	}
	snaps, err := versions.ListVersionQuestions(ctx, current.ID)
	if err != nil {
		t.Fatalf("list snaps: %v", err)
	}
	if len(snaps) != 3 {
		t.Fatalf("want 3 snapshots, got %d", len(snaps))
	}
	for _, s := range snaps {
		if s.Content != "题干 v0" || s.Answer != `"A"` {
			t.Fatalf("snapshot not frozen: %+v", s)
		}
	}
}

// TestDuplicatePublishFails verifies repeat publish takes effect only once.
func TestDuplicatePublishFails(t *testing.T) {
	svc, _, _, questions := newExamServiceForTest()
	seedQuestions(questions, 3, constants.QuestionSingle, "easy")
	ctx := context.Background()
	created, err := svc.Create(ctx, 1, createReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Publish(ctx, constants.RoleAdmin, 1, created.ID); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := svc.Publish(ctx, constants.RoleAdmin, 1, created.ID); err == nil {
		t.Fatal("second publish must fail with conflict")
	}
}

// TestRegenerateCreatesNewVersion verifies old versions survive regeneration
// and remain readable, while a pending draft blocks concurrent regeneration.
func TestRegenerateCreatesNewVersionAndKeepsOld(t *testing.T) {
	svc, _, versions, questions := newExamServiceForTest()
	seedQuestions(questions, 3, constants.QuestionSingle, "easy")
	ctx := context.Background()
	created, err := svc.Create(ctx, 1, createReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Publish(ctx, constants.RoleAdmin, 1, created.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	before, err := versions.FindCurrentVersion(ctx, created.ID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	newDraft, err := svc.Regenerate(ctx, constants.RoleAdmin, 1, created.ID, regenRequest())
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if newDraft.VersionNo != 2 || newDraft.Status != constants.VersionDraft {
		t.Fatalf("unexpected new draft: %+v", newDraft)
	}

	// Old version v1 must still be published and readable.
	old, err := versions.FindVersionByExamAndNo(ctx, created.ID, 1)
	if err != nil {
		t.Fatalf("find old version: %v", err)
	}
	if old.Status != constants.VersionPublished {
		t.Fatalf("old version status = %s, want published", old.Status)
	}
	oldSnaps, err := versions.ListVersionQuestions(ctx, before.ID)
	if err != nil || len(oldSnaps) != 3 {
		t.Fatalf("old snapshots must remain readable: %v %d", err, len(oldSnaps))
	}

	// Concurrent regeneration while a draft is pending must be rejected.
	if _, err := svc.Regenerate(ctx, constants.RoleAdmin, 1, created.ID, regenRequest()); err == nil {
		t.Fatal("second concurrent regeneration must conflict")
	}

	// Publishing the draft archives the old version.
	if err := svc.Publish(ctx, constants.RoleAdmin, 1, created.ID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	oldAfter, _ := versions.FindVersionByExamAndNo(ctx, created.ID, 1)
	if oldAfter.Status != constants.VersionArchived {
		t.Fatalf("old version should be archived, got %s", oldAfter.Status)
	}
}
