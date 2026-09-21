package repository

import "testing"

func TestNormalizePage(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{"defaults", 0, 0, 1, 10},
		{"negative defaults", -1, -5, 1, 10},
		{"clamp max", 1, 500, 1, 100},
		{"keep valid", 3, 25, 3, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotSize := NormalizePage(tt.page, tt.pageSize)
			if gotPage != tt.wantPage || gotSize != tt.wantPageSize {
				t.Fatalf("NormalizePage(%d, %d) = (%d, %d), want (%d, %d)",
					tt.page, tt.pageSize, gotPage, gotSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestOffset(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		want     int
	}{
		{"first page", 1, 10, 0},
		{"second page", 2, 10, 10},
		{"clamped page", 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Offset(tt.page, tt.pageSize); got != tt.want {
				t.Fatalf("Offset(%d, %d) = %d, want %d", tt.page, tt.pageSize, got, tt.want)
			}
		})
	}
}
