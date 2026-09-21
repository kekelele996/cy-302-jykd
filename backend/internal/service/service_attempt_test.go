package service

import "testing"

func TestIsCorrectObjective(t *testing.T) {
	tests := []struct {
		name    string
		qtype   string
		correct any
		student any
		want    bool
	}{
		{"single correct", "single", "A", "A", true},
		{"single wrong", "single", "A", "B", false},
		{"true false case insensitive", "true_false", "T", "t", true},
		{"true false wrong", "true_false", "T", "F", false},
		{"multiple exact set", "multiple", []any{"A", "C"}, []any{"C", "A"}, true},
		{"multiple subset", "multiple", []any{"A", "C"}, []any{"A"}, false},
		{"multiple extra", "multiple", []any{"A"}, []any{"A", "B"}, false},
		{"unknown type never correct", "short_answer", "anything", "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCorrectObjective(tt.qtype, tt.correct, tt.student); got != tt.want {
				t.Fatalf("isCorrectObjective(%q, %v, %v) = %v, want %v", tt.qtype, tt.correct, tt.student, got, tt.want)
			}
		})
	}
}
