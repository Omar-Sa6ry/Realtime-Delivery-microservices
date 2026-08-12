package domain

// QuotaUsage tracks a user's storage and upload slot consumption.
type QuotaUsage struct {
	UserID          string `json:"userId"`
	UsedBytes       int64  `json:"usedBytes"`
	QuotaBytes      int64  `json:"quotaBytes"`
	ActiveUploads   int    `json:"activeUploads"`
	MaxConcurrent   int    `json:"maxConcurrent"`
}

// CanUpload returns true when the user has sufficient quota for a file of the given size.
func (q *QuotaUsage) CanUpload(fileSize int64) bool {
	return q.UsedBytes+fileSize <= q.QuotaBytes
}

// HasUploadSlot returns true when the user has not reached the concurrent upload limit.
func (q *QuotaUsage) HasUploadSlot() bool {
	return q.ActiveUploads < q.MaxConcurrent
}

// RemainingBytes returns the number of bytes still available for the user.
func (q *QuotaUsage) RemainingBytes() int64 {
	remaining := q.QuotaBytes - q.UsedBytes
	if remaining < 0 {
		return 0
	}
	return remaining
}
