package domain

type QuotaUsage struct {
	UserID          string `json:"userId"`
	UsedBytes       int64  `json:"usedBytes"`
	QuotaBytes      int64  `json:"quotaBytes"`
	ActiveUploads   int    `json:"activeUploads"`
	MaxConcurrent   int    `json:"maxConcurrent"`
}

func (q *QuotaUsage) CanUpload(fileSize int64) bool {
	return q.UsedBytes+fileSize <= q.QuotaBytes
}

func (q *QuotaUsage) HasUploadSlot() bool {
	return q.ActiveUploads < q.MaxConcurrent
}

func (q *QuotaUsage) RemainingBytes() int64 {
	remaining := q.QuotaBytes - q.UsedBytes
	if remaining < 0 {
		return 0
	}
	return remaining
}
