package domain

import "time"

// UploadStatus represents the status of an upload session.
type UploadStatus string

const (
	UploadStatusPending   UploadStatus = "PENDING"
	UploadStatusUploading UploadStatus = "UPLOADING"
	UploadStatusCompleted UploadStatus = "COMPLETED"
	UploadStatusAborted   UploadStatus = "ABORTED"
	UploadStatusExpired   UploadStatus = "EXPIRED"
)

// UploadPart represents a single S3 multipart upload part.
type UploadPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"eTag"`
}

type UploadSession struct {
	UploadID       string       `json:"uploadId"`
	MediaID        string       `json:"mediaId"`
	UserID         string       `json:"userId"`
	S3UploadID     string       `json:"s3UploadId"`
	ObjectKey      string       `json:"objectKey"`
	TotalParts     int          `json:"totalParts"`
	PartSize       int64        `json:"partSize"`
	CompletedParts []UploadPart `json:"completedParts"`
	Status         UploadStatus `json:"status"`
	ExpiresAt      time.Time    `json:"expiresAt"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

// IsExpired reports whether the upload session TTL has elapsed.
func (s *UploadSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// MissingPartNumbers returns the list of part numbers not yet completed.
func (s *UploadSession) MissingPartNumbers() []int {
	completed := make(map[int]struct{}, len(s.CompletedParts))
	for _, p := range s.CompletedParts {
		completed[p.PartNumber] = struct{}{}
	}
	missing := make([]int, 0)
	for i := 1; i <= s.TotalParts; i++ {
		if _, ok := completed[i]; !ok {
			missing = append(missing, i)
		}
	}
	return missing
}
