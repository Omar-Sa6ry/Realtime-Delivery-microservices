package domain

import "time"

type MediaStatus string

const (
	MediaStatusPending     MediaStatus = "PENDING"
	MediaStatusUploading   MediaStatus = "UPLOADING"
	MediaStatusUploaded    MediaStatus = "UPLOADED"
	MediaStatusScanning    MediaStatus = "SCANNING"
	MediaStatusProcessing  MediaStatus = "PROCESSING"
	MediaStatusReady       MediaStatus = "READY"
	MediaStatusFailed      MediaStatus = "FAILED"
	MediaStatusQuarantined MediaStatus = "QUARANTINED"
	MediaStatusDeleting    MediaStatus = "DELETING"
	MediaStatusDeleted     MediaStatus = "DELETED"
	MediaStatusAborted     MediaStatus = "ABORTED"
)

var allowedTransitions = map[MediaStatus][]MediaStatus{
	MediaStatusPending:     {MediaStatusUploading, MediaStatusFailed, MediaStatusAborted},
	MediaStatusUploading:   {MediaStatusUploaded, MediaStatusAborted, MediaStatusFailed},
	MediaStatusUploaded:    {MediaStatusScanning, MediaStatusFailed},
	MediaStatusScanning:    {MediaStatusProcessing, MediaStatusQuarantined, MediaStatusFailed},
	MediaStatusProcessing:  {MediaStatusReady, MediaStatusFailed},
	MediaStatusReady:       {MediaStatusDeleting},
	MediaStatusFailed:      {MediaStatusDeleting},
	MediaStatusQuarantined: {MediaStatusDeleting},
	MediaStatusDeleting:    {MediaStatusDeleted, MediaStatusFailed},
	// Terminal states — no transitions allowed.
	MediaStatusDeleted: {},
	MediaStatusAborted: {},
}

type MediaType string

const (
	MediaTypeImage    MediaType = "IMAGE"
	MediaTypeVideo    MediaType = "VIDEO"
	MediaTypeDocument MediaType = "DOCUMENT"
	MediaTypeOther    MediaType = "OTHER"
)

type Media struct {
	MediaID     string      `json:"mediaId"`
	OwnerID     string      `json:"ownerId"`
	DeliveryID  string      `json:"deliveryId,omitempty"`
	FileName    string      `json:"fileName"`
	ContentType string      `json:"contentType"`
	MediaType   MediaType   `json:"mediaType"`
	Size        int64       `json:"size"`
	Checksum    string      `json:"checksum,omitempty"` // SHA-256 hex, client-provided
	ObjectKey   string      `json:"objectKey"`
	Status      MediaStatus `json:"status"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

func (m *Media) CanTransitionTo(next MediaStatus) bool {
	allowed, ok := allowedTransitions[m.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}
