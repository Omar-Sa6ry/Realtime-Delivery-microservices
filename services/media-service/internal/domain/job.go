package domain

import "time"

type JobType string

const (
	JobTypeScan               JobType = "SCAN"
	JobTypeCompress           JobType = "COMPRESS"
	JobTypeThumbnail          JobType = "THUMBNAIL"
	JobTypeTranscode          JobType = "TRANSCODE"
	JobTypeMetadataExtraction JobType = "METADATA_EXTRACTION"
	JobTypeDelete             JobType = "DELETE"
	JobTypeImageProcess       JobType = "IMAGE_PROCESS"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusDLQ       JobStatus = "DLQ"
)

type MediaJob struct {
	JobID       string    `json:"jobId"`
	MediaID     string    `json:"mediaId"`
	JobType     JobType   `json:"jobType"`
	Status      JobStatus `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"maxAttempts"`
	LastError   string    `json:"lastError,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (j *MediaJob) IsRetryable() bool {
	return j.Attempts < j.MaxAttempts
}
