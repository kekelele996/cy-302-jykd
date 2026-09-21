package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
)

func errWrap(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}

func errWrapf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// drawPaperItems selects questions from the bank according to the paper
// configuration. Returns the working-copy rows (ExamID left unset) and the
// computed total score.
func (s *ExamService) drawPaperItems(ctx context.Context, configs []dto.PaperQuestionConfig, declaredTotal float64) ([]model.ExamQuestion, float64, error) {
	var computedTotal float64
	var items []model.ExamQuestion
	order := 0
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, cfg := range configs {
		questions, err := s.questionRepo.ListQuestionsByTypeDifficulty(ctx, cfg.Type, cfg.Difficulty)
		if err != nil {
			return nil, 0, errWrap("list questions by type difficulty", err)
		}
		if len(questions) < cfg.Count {
			return nil, 0, errWrapf("%w: 题型 %s 难度 %s 题库数量不足（需要 %d，实际 %d）",
				ErrValidation, cfg.Type, cfg.Difficulty, cfg.Count, len(questions))
		}
		shuffle(questions, rng)
		for i := 0; i < cfg.Count; i++ {
			items = append(items, model.ExamQuestion{
				QuestionID: questions[i].ID,
				Score:      cfg.Score,
				SortOrder:  order,
			})
			computedTotal += cfg.Score
			order++
		}
	}

	if declaredTotal > 0 && declaredTotal != computedTotal {
		return nil, 0, errWrapf("%w: 总分 %.2f 与各题型分值之和 %.2f 不一致", ErrValidation, declaredTotal, computedTotal)
	}
	return items, computedTotal, nil
}
