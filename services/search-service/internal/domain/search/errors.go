package search

import "errors"

var (
	ErrInvalidQuery     = errors.New("search query is invalid or exceeds maximum length")
	ErrInvalidCursor    = errors.New("invalid pagination cursor")
	ErrIndexNotFound    = errors.New("search index not found")
	ErrDocumentNotFound = errors.New("document not found")
	ErrVersionConflict  = errors.New("document version is older than or equal to current index version")
	ErrSearchUnavailable= errors.New("search engine is temporarily unavailable")
	ErrUnauthorized     = errors.New("unauthorized search operation")
	ErrReindexInProgress= errors.New("reindex job is already in progress for this index")
)
