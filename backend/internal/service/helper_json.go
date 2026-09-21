package service

import (
	"encoding/json"
	"fmt"

	"github.com/gbexam/online-exam/internal/dto"
)

func marshalOptions(options []dto.Option) (string, error) {
	if options == nil {
		options = []dto.Option{}
	}
	b, err := json.Marshal(options)
	if err != nil {
		return "", fmt.Errorf("marshal options: %w", err)
	}
	return string(b), nil
}

func unmarshalOptions(raw string) ([]dto.Option, error) {
	if raw == "" {
		return []dto.Option{}, nil
	}
	var options []dto.Option
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, fmt.Errorf("unmarshal options: %w", err)
	}
	return options, nil
}

func marshalAnswer(answer any) (string, error) {
	b, err := json.Marshal(answer)
	if err != nil {
		return "", fmt.Errorf("marshal answer: %w", err)
	}
	return string(b), nil
}

func unmarshalAnswer(raw string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	var answer any
	if err := json.Unmarshal([]byte(raw), &answer); err != nil {
		return nil, fmt.Errorf("unmarshal answer: %w", err)
	}
	return answer, nil
}
