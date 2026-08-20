package kafka

const (
	TopicUploadCreated       = "media.upload.created"
	TopicUploadCompleted     = "media.upload.completed"
	TopicUploadAborted       = "media.upload.aborted"

	TopicScanStarted         = "media.scan.started"
	TopicScanCompleted       = "media.scan.completed"
	TopicScanFailed          = "media.scan.failed"

	TopicProcessingStarted   = "media.processing.started"
	TopicProcessingCompleted = "media.processing.completed"
	TopicProcessingFailed    = "media.processing.failed"

	TopicMediaReady          = "media.ready"

	TopicDeleteRequested     = "media.delete.requested"
	TopicMediaDeleted        = "media.deleted"
	TopicDeleteFailed        = "media.delete.failed"
)

// ConsumerGroups defines independent consumer group IDs per worker type.
const (
	GroupScanner            = "media-scanner"
	GroupImageWorker        = "media-image-worker"
	GroupVideoWorker        = "media-video-worker"
	GroupDeleteWorker       = "media-delete-worker"
	GroupCompressionWorker  = "media-compression-worker"
	GroupMetadataWorker     = "media-metadata-worker"
	GroupNotification       = "media-notification"
)

