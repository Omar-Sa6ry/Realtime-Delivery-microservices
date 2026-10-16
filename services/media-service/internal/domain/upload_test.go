package domain

import (
	"testing"
	"time"
)

func TestUploadSession_IsExpired(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"past", now.Add(-time.Minute), true},
		{"future", now.Add(time.Minute), false},
		{"zero_time", time.Time{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &UploadSession{ExpiresAt: tc.expiresAt}
			if got := s.IsExpired(); got != tc.want {
				t.Errorf("IsExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func intsEqual(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestUploadSession_MissingPartNumbers(t *testing.T) {
	cases := []struct {
		name        string
		totalParts  int
		completed   []UploadPart
		wantMissing []int
	}{
		{
			name:        "no_parts_completed",
			totalParts:  3,
			completed:   nil,
			wantMissing: []int{1, 2, 3},
		},
		{
			name:        "partially_completed",
			totalParts:  5,
			completed:   []UploadPart{{PartNumber: 1}, {PartNumber: 3}, {PartNumber: 5}},
			wantMissing: []int{2, 4},
		},
		{
			name:        "fully_completed",
			totalParts:  3,
			completed:   []UploadPart{{PartNumber: 1}, {PartNumber: 2}, {PartNumber: 3}},
			wantMissing: []int{},
		},
		{
			name:        "zero_total_parts",
			totalParts:  0,
			completed:   nil,
			wantMissing: []int{},
		},
		{
			name:        "duplicate_completed_parts",
			totalParts:  2,
			completed:   []UploadPart{{PartNumber: 1}, {PartNumber: 1}},
			wantMissing: []int{2},
		},
		{
			name:        "out_of_range_parts_ignored",
			totalParts:  2,
			completed:   []UploadPart{{PartNumber: 0}, {PartNumber: 5}},
			wantMissing: []int{1, 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &UploadSession{TotalParts: tc.totalParts, CompletedParts: tc.completed}
			got := s.MissingPartNumbers()
			if !intsEqual(got, tc.wantMissing) {
				t.Errorf("MissingPartNumbers() = %v, want %v", got, tc.wantMissing)
			}
		})
	}
}
