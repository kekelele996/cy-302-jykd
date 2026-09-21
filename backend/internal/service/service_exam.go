package service

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// ExamService handles exam creation, paper generation, version freezing and
// lifecycle. Published content is immutable: publish and regroup copy the exact
// question bank state into paper_versions, and all reads during/after an exam
// resolve through the version bound at publish/start time.
type ExamService struct {
	baseService
	repo         ExamRepo
	paperRepo    PaperRepo
	questionRepo QuestionRepo
}

// NewExamService constructs ExamService.
func NewExamService(repo ExamRepo, paperRepo PaperRepo, questionRepo QuestionRepo, logger *slog.Logger) *ExamService {
	return &ExamService{
		baseService:  NewBaseService(logger),
		repo:         repo,
		paperRepo:    paperRepo,
		questionRepo: questionRepo,
	}
}

// Create builds an exam and auto-generates its draft working paper.
func (s *ExamService) Create(ctx context.Context, createdBy uint, req dto.ExamCreateRequest) (*dto.ExamResponse, error) {
	items, computedTotal, err := s.drawPaperItems(ctx, req.QuestionConfig, req.TotalScore)
	if err != nil {
		return nil, err
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
		return nil, errWrap("create exam", err)
	}
	for i := range items {
		items[i].ExamID = exam.ID
	}
	if err := s.repo.ReplaceExamQuestions(ctx, exam.ID, items); err != nil {
		return nil, errWrap("replace exam questions", err)
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
		return dto.PageResult{}, errWrap("list exams", err)
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

// Publish freezes the current draft paper as version 1 and opens the exam.
//
// Concurrency: the exam row is locked for the duration of the transaction and
// the status flip is conditional on status = 'draft'. Two simultaneous publish
// requests therefore serialize; the loser sees an already published exam and
// the call is a no-op (idempotent), so the snapshot is created exactly once.
func (s *ExamService) Publish(ctx context.Context, role string, userID, id uint) (*dto.PaperVersionSummary, error) {
	var summary *dto.PaperVersionSummary
	err := s.paperRepo.WithTransaction(ctx, func(tx *gorm.DB) error {
		exam, err := s.paperRepo.LockExamForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if role == constants.RoleTeacher && exam.CreatedBy != userID {
			return ErrForbidden
		}

		// Already published/closed: repeated publish is idempotent and returns
		// the existing frozen version without creating another one.
		if exam.CurrentVersionID != 0 && exam.Status != constants.ExamDraft {
			version, vErr := s.paperRepo.FindPaperVersionByID(ctx, exam.CurrentVersionID)
			if vErr != nil {
				return vErr
			}
			frozen, lErr := s.paperRepo.ListPaperVersionQuestions(ctx, version.ID)
			if lErr != nil {
				return lErr
			}
			summary = versionSummary(version, len(frozen))
			summary.IsCurrent = true
			return nil
		}

		count, err := s.repo.CountExamQuestionsTx(ctx, tx, id)
		if err != nil {
			return errWrap("count exam questions", err)
		}
		if count == 0 {
			return errWrapf("%w: 试卷没有题目，无法发布", ErrValidation)
		}

		version, err := s.freezeCurrentDraft(ctx, tx, exam, 1)
		if err != nil {
			return err
		}
		affected, err := s.repo.PublishExamIfDraft(ctx, tx, exam.ID, version.ID)
		if err != nil {
			return err
		}
		if affected == 0 {
			// Concurrent publish won the race; roll back our snapshot by
			// returning ErrConflict, and the whole transaction is discarded.
			return ErrConflict
		}
		frozen, err := s.paperRepo.ListPaperVersionQuestions(ctx, version.ID)
		if err != nil {
			return err
		}
		summary = versionSummary(version, len(frozen))
		summary.IsCurrent = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// freezeCurrentDraft copies the working-copy exam questions (and the live
// question rows, locked in the same transaction) into a new immutable version.
func (s *ExamService) freezeCurrentDraft(ctx context.Context, tx *gorm.DB, exam *model.Exam, nextVersionNo int) (*model.PaperVersion, error) {
	items, err := s.repo.ListExamQuestionsTx(ctx, tx, exam.ID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errWrapf("%w: 试卷没有题目，无法冻结版本", ErrValidation)
	}
	ids := make([]uint, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDsTx(ctx, tx, ids)
	if err != nil {
		return nil, err
	}

	in := repository.FreezePaperInput{
		ExamID:    exam.ID,
		VersionNo: nextVersionNo,
		Title:     exam.Title,
		CreatedBy: exam.CreatedBy,
		Items:     make([]repository.FreezePaperItem, 0, len(items)),
	}
	for _, it := range items {
		q, ok := questionMap[it.QuestionID]
		if !ok {
			return nil, errWrapf("%w: 题目 %d 已从题库删除，请重新组卷后再发布", ErrValidation, it.QuestionID)
		}
		in.Items = append(in.Items, repository.FreezePaperItem{
			QuestionID:     q.ID,
			Type:           q.Type,
			Content:        q.Content,
			Options:        q.Options,
			Answer:         q.Answer,
			Analysis:       q.Analysis,
			Difficulty:     q.Difficulty,
			KnowledgePoint: q.KnowledgePoint,
			Score:          it.Score,
			SortOrder:      it.SortOrder,
		})
	}
	version, err := s.paperRepo.CreatePaperVersion(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	return version, nil
}

// Regroup re-generates the paper of an already published/closed exam.
//
// A brand-new immutable version is created (old versions stay readable) and the
// exam's current pointer moves to it. In-progress attempts are unaffected
// because they are bound to their own paper_version_id. ExpectedRevision is an
// optimistic-lock token: a concurrent regroup/publish that committed first
// bumps the revision and the stale request fails with ErrConflict, so only one
// of two simultaneous changes takes effect.
func (s *ExamService) Regroup(ctx context.Context, role string, userID, id uint, req dto.RegroupRequest) (*dto.RegroupResponse, error) {
	items, computedTotal, err := s.drawPaperItems(ctx, req.QuestionConfig, 0)
	if err != nil {
		return nil, err
	}

	var resp *dto.RegroupResponse
	err = s.paperRepo.WithTransaction(ctx, func(tx *gorm.DB) error {
		exam, err := s.paperRepo.LockExamForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if role == constants.RoleTeacher && exam.CreatedBy != userID {
			return ErrForbidden
		}
		if exam.Status == constants.ExamDraft {
			return errWrapf("%w: 草稿考试请直接删除后重新创建，或先发布", ErrValidation)
		}
		if exam.Revision != req.ExpectedRevision {
			return ErrConflict
		}
		if exam.CurrentVersionID == 0 {
			return errWrapf("%w: 考试尚未冻结版本，无法重新组卷", ErrValidation)
		}

		nextNo, err := s.paperRepo.MaxPaperVersionNo(ctx, tx, exam.ID)
		if err != nil {
			return err
		}
		nextNo++

		// Update the working copy for future draft-style inspection, then
		// immediately freeze the new immutable version.
		for i := range items {
			items[i].ExamID = exam.ID
		}
		if err := s.repo.ReplaceExamQuestionsTx(ctx, tx, exam.ID, items); err != nil {
			return err
		}
		exam.TotalScore = computedTotal
		version, err := s.freezeCurrentDraft(ctx, tx, exam, nextNo)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateExamTotalScoreTx(ctx, tx, exam.ID, computedTotal); err != nil {
			return err
		}
		if err := s.paperRepo.PointExamToVersion(ctx, tx, exam.ID, version.ID, exam.Status); err != nil {
			return err
		}
		resp = &dto.RegroupResponse{
			VersionID:     version.ID,
			VersionNo:     version.VersionNo,
			Revision:      exam.Revision + 1,
			TotalScore:    computedTotal,
			QuestionCount: len(items),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
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
		return errWrap("close exam", err)
	}
	return nil
}

// Delete removes an exam and all its frozen versions (only creator/admin).
func (s *ExamService) Delete(ctx context.Context, role string, userID, id uint) error {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return ErrForbidden
	}
	if err := s.repo.DeleteExam(ctx, id); err != nil {
		return errWrap("delete exam", err)
	}
	return nil
}

// ListVersions returns all frozen versions of an exam, newest first.
func (s *ExamService) ListVersions(ctx context.Context, role string, userID, id uint) ([]dto.PaperVersionSummary, error) {
	exam, err := s.repo.FindExamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	versions, err := s.paperRepo.ListPaperVersions(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]dto.PaperVersionSummary, 0, len(versions))
	for i := range versions {
		v := &versions[i]
		frozen, err := s.paperRepo.ListPaperVersionQuestions(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		summary := versionSummary(v, len(frozen))
		summary.IsCurrent = v.ID == exam.CurrentVersionID
		result = append(result, *summary)
	}
	return result, nil
}

// ListPaperQuestions returns the paper content for staff review.
//
// versionNo = 0 means the currently published version (or the editable draft
// for draft exams); a positive number returns that historical frozen version.
// Every read path (staff review, student taking, attempt review, grading,
// report) resolves the same snapshot, so a page refresh always shows identical
// content.
func (s *ExamService) ListPaperQuestions(ctx context.Context, role string, userID, id uint, versionNo int) ([]dto.ExamQuestionResponse, error) {
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

	if versionNo > 0 || (versionNo == 0 && exam.CurrentVersionID != 0 && exam.Status != constants.ExamDraft) {
		version, frozen, err := s.resolveVersion(ctx, exam, versionNo)
		if err != nil {
			return nil, err
		}
		return frozenToResponses(version, frozen), nil
	}

	// Draft exam: serve the editable working copy joined to live questions.
	items, err := s.repo.ListExamQuestions(ctx, id)
	if err != nil {
		return nil, errWrap("list exam questions", err)
	}
	ids := make([]uint, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.QuestionID)
	}
	questionMap, err := s.questionRepo.FindQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ExamQuestionResponse, 0, len(items))
	for _, it := range items {
		q, ok := questionMap[it.QuestionID]
		if !ok {
			continue
		}
		result = append(result, dto.ExamQuestionResponse{
			ID:        it.ID,
			Score:     it.Score,
			Question:  *questionToResponse(&q),
		})
	}
	return result, nil
}

// resolveVersion loads a frozen version by explicit number, or the current one.
func (s *ExamService) resolveVersion(ctx context.Context, exam *model.Exam, versionNo int) (*model.PaperVersion, []model.PaperVersionQuestion, error) {
	var version *model.PaperVersion
	var err error
	if versionNo > 0 {
		versions, listErr := s.paperRepo.ListPaperVersions(ctx, exam.ID)
		if listErr != nil {
			return nil, nil, listErr
		}
		for i := range versions {
			if versions[i].VersionNo == versionNo {
				version = &versions[i]
				break
			}
		}
		if version == nil {
			return nil, nil, errWrapf("%w: 试卷版本 v%d 不存在", ErrNotFound, versionNo)
		}
	} else {
		version, err = s.paperRepo.FindPaperVersionByID(ctx, exam.CurrentVersionID)
		if err != nil {
			return nil, nil, err
		}
	}
	frozen, err := s.paperRepo.ListPaperVersionQuestions(ctx, version.ID)
	if err != nil {
		return nil, nil, err
	}
	return version, frozen, nil
}

// CountQuestions exposes question count for exam metadata.
func (s *ExamService) CountQuestions(ctx context.Context, id uint) (int64, error) {
	return s.repo.CountExamQuestions(ctx, id)
}

func (s *ExamService) toResponse(ctx context.Context, exam *model.Exam) (*dto.ExamResponse, error) {
	count, err := s.repo.CountExamQuestions(ctx, exam.ID)
	if err != nil {
		return nil, errWrap("count exam questions", err)
	}
	resp := &dto.ExamResponse{
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
		Revision:         exam.Revision,
		CreatedBy:        exam.CreatedBy,
		CreatedAt:        exam.CreatedAt,
	}
	if exam.CurrentVersionID != 0 {
		if version, vErr := s.paperRepo.FindPaperVersionByID(ctx, exam.CurrentVersionID); vErr == nil {
			resp.CurrentVersionNo = version.VersionNo
		}
		if versions, lErr := s.paperRepo.ListPaperVersions(ctx, exam.ID); lErr == nil {
			resp.VersionCount = len(versions)
		}
	}
	return resp, nil
}

// versionSummary maps a frozen version model to its summary DTO.
func versionSummary(v *model.PaperVersion, questionCount int) *dto.PaperVersionSummary {
	return &dto.PaperVersionSummary{
		ID:            v.ID,
		VersionNo:     v.VersionNo,
		Title:         v.Title,
		TotalScore:    v.TotalScore,
		QuestionCount: questionCount,
		SnapshotAt:    v.SnapshotAt,
		CreatedBy:     v.CreatedBy,
	}
}

// frozenQuestionToResponse converts a snapshot row to a question response.
func frozenQuestionToResponse(q *model.PaperVersionQuestion) dto.QuestionResponse {
	options, _ := unmarshalOptions(q.Options)
	answer, _ := unmarshalAnswer(q.Answer)
	return dto.QuestionResponse{
		ID:             q.QuestionID,
		Type:           q.Type,
		Content:        q.Content,
		Options:        options,
		Answer:         answer,
		Analysis:       q.Analysis,
		Difficulty:     q.Difficulty,
		KnowledgePoint: q.KnowledgePoint,
		Score:          q.Score,
	}
}

func frozenToResponses(version *model.PaperVersion, frozen []model.PaperVersionQuestion) []dto.ExamQuestionResponse {
	result := make([]dto.ExamQuestionResponse, 0, len(frozen))
	for i := range frozen {
		q := &frozen[i]
		result = append(result, dto.ExamQuestionResponse{
			ID:        q.ID,
			Score:     q.Score,
			VersionID: version.ID,
			VersionNo: version.VersionNo,
			Question:  frozenQuestionToResponse(q),
		})
	}
	return result
}

