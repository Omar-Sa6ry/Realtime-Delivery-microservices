package domain

import "time"

// JobType identifies the kind of background processing job.
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

// JobStatus tracks the lifecycle of a background job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusDLQ       JobStatus = "DLQ"
)

// MediaJob represents a single background processing task for a media item.
// Jobs are created in DynamoDB and picked up by Kafka consumers (workers).
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

// IsRetryable returns true if the job can be retried (attempts < maxAttempts).
func (j *MediaJob) IsRetryable() bool {
	return j.Attempts < j.MaxAttempts
}

// DefaultMaxAttempts returns the default retry count per job type.
func DefaultMaxAttempts(jt JobType) int {
	switch jt {
	case JobTypeTranscode:
		return 2 // video transcoding is expensive; limit retries
	case JobTypeScan:
		return 5
	default:
		return 3
	}
}
