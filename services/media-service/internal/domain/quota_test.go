package domain

import "testing"

func TestQuotaUsage_CanUpload(t *testing.T) {
	cases := []struct {
		name     string
		used     int64
		quota    int64
		fileSize int64
		want     bool
	}{
		{"fits_exactly", 50, 100, 50, true},
		{"exceeds_by_one", 50, 100, 51, false},
		{"well_within", 0, 100, 10, true},
		{"already_full", 100, 100, 1, false},
		{"empty_quota", 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &QuotaUsage{UsedBytes: tc.used, QuotaBytes: tc.quota}
			if got := q.CanUpload(tc.fileSize); got != tc.want {
				t.Errorf("CanUpload(%d) = %v, want %v", tc.fileSize, got, tc.want)
			}
		})
	}
}

func TestQuotaUsage_HasUploadSlot(t *testing.T) {
	cases := []struct {
		name          string
		activeUploads int
		maxConcurrent int
		want          bool
	}{
		{"slot_available", 0, 5, true},
		{"last_slot", 4, 5, true},
		{"no_slot", 5, 5, false},
		{"over_limit", 6, 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &QuotaUsage{ActiveUploads: tc.activeUploads, MaxConcurrent: tc.maxConcurrent}
			if got := q.HasUploadSlot(); got != tc.want {
				t.Errorf("HasUploadSlot() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestQuotaUsage_RemainingBytes(t *testing.T) {
	cases := []struct {
		name  string
		used  int64
		quota int64
		want  int64
	}{
		{"partial_usage", 30, 100, 70},
		{"no_usage", 0, 100, 100},
		{"exact", 100, 100, 0},
		{"overused_clamped_to_zero", 150, 100, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &QuotaUsage{UsedBytes: tc.used, QuotaBytes: tc.quota}
			if got := q.RemainingBytes(); got != tc.want {
				t.Errorf("RemainingBytes() = %d, want %d", got, tc.want)
			}
		})
	}
}
