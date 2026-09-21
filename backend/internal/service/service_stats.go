package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
)

// StatsService builds dashboard and exam statistics.
type StatsService struct {
	baseService
	repo StatsRepo
}

// NewStatsService constructs StatsService.
func NewStatsService(repo StatsRepo, logger *slog.Logger) *StatsService {
	return &StatsService{baseService: NewBaseService(logger), repo: repo}
}

// Overview returns aggregate counts for the dashboard.
func (s *StatsService) Overview(ctx context.Context) (*dto.OverviewResponse, error) {
	users, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	questions, err := s.repo.CountQuestions(ctx)
	if err != nil {
		return nil, fmt.Errorf("count questions: %w", err)
	}
	exams, err := s.repo.CountExams(ctx)
	if err != nil {
		return nil, fmt.Errorf("count exams: %w", err)
	}
	attempts, err := s.repo.CountAttempts(ctx)
	if err != nil {
		return nil, fmt.Errorf("count attempts: %w", err)
	}
	return &dto.OverviewResponse{
		UserCount:     users,
		QuestionCount: questions,
		ExamCount:     exams,
		AttemptCount:  attempts,
	}, nil
}

// ExamStats returns score statistics and ranking for one exam.
func (s *StatsService) ExamStats(ctx context.Context, role string, userID, examID uint) (*dto.ExamStatResponse, error) {
	exam, err := s.repo.FindExamByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	attempts, err := s.repo.ListAttemptsByExam(ctx, examID)
	if err != nil {
		return nil, fmt.Errorf("list attempts by exam: %w", err)
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
		if submitted[i].SubmittedAt != nil && submitted[j].SubmittedAt != nil {
			return submitted[i].SubmittedAt.Before(*submitted[j].SubmittedAt)
		}
		return submitted[i].ID < submitted[j].ID
	})

	total := 0.0
	highest := 0.0
	lowest := 0.0
	passCount := 0
	if len(submitted) > 0 {
		highest = submitted[0].TotalScore
		lowest = submitted[len(submitted)-1].TotalScore
	}
	for _, a := range submitted {
		total += a.TotalScore
		if exam.TotalScore > 0 && a.TotalScore >= exam.TotalScore*0.6 {
			passCount++
		}
	}
	average := 0.0
	if len(submitted) > 0 {
		average = total / float64(len(submitted))
	}

	buckets := buildScoreBuckets(submitted, exam.TotalScore)
	ranking := make([]dto.RankItem, 0, len(submitted))
	for i, a := range submitted {
		name := ""
		username := ""
		if user, userErr := s.repo.FindUserByID(ctx, a.StudentID); userErr == nil {
			name = user.Name
			username = user.Username
		}
		ranking = append(ranking, dto.RankItem{
			Rank:            i + 1,
			StudentName:     name,
			StudentUsername: username,
			TotalScore:      a.TotalScore,
			SubmittedAt:     a.SubmittedAt,
		})
	}

	return &dto.ExamStatResponse{
		ExamID:            exam.ID,
		ExamTitle:         exam.Title,
		ParticipantCount:  len(submitted),
		AverageScore:      round2(average),
		HighestScore:      highest,
		LowestScore:       lowest,
		PassCount:         passCount,
		ScoreDistribution: buckets,
		Ranking:           ranking,
	}, nil
}

func buildScoreBuckets(attempts []model.ExamAttempt, totalScore float64) []dto.ScoreBucket {
	labels := []string{"0-59", "60-69", "70-79", "80-89", "90-100"}
	buckets := make([]dto.ScoreBucket, len(labels))
	for i, label := range labels {
		buckets[i] = dto.ScoreBucket{Label: label, Count: 0}
	}
	for _, a := range attempts {
		percent := 100.0
		if totalScore > 0 {
			percent = a.TotalScore / totalScore * 100
		}
		switch {
		case percent < 60:
			buckets[0].Count++
		case percent < 70:
			buckets[1].Count++
		case percent < 80:
			buckets[2].Count++
		case percent < 90:
			buckets[3].Count++
		default:
			buckets[4].Count++
		}
	}
	return buckets
}
