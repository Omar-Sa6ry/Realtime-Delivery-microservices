package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

var (
	_ ports.MediaRepository   = (*MockMediaRepository)(nil)
	_ ports.UploadRepository  = (*MockUploadRepository)(nil)
	_ ports.JobRepository     = (*MockJobRepository)(nil)
	_ ports.OutboxRepository  = (*MockOutboxRepository)(nil)
	_ ports.QuotaRepository   = (*MockQuotaRepository)(nil)
	_ ports.VersionRepository = (*MockVersionRepository)(nil)
)

type MediaStatusCall struct {
	MediaID string
	Current domain.MediaStatus
	Next    domain.MediaStatus
}

type MockMediaRepository struct {
	mu sync.Mutex

	Media *domain.Media
	GetErr error
	CreateErr error
	UpdateStatusErr error
	OnUpdateStatus func(call MediaStatusCall) error
	ListErr error
	DeleteErr error

	ListResults []*domain.Media
	ListCursor string

	// Stored items keyed by media id.
	Store map[string]*domain.Media

	// Recorded calls.
	Created        []*domain.Media
	UpdateStatuses []MediaStatusCall
	ListOwnerCalls []ListOwnerCall
	Deleted        []string
	GetByIDCalls   []string
}

// ListOwnerCall records a ListByOwner invocation.
type ListOwnerCall struct {
	OwnerID string
	Limit   int
	Cursor  string
}

// NewMockMediaRepository returns an empty repository.
func NewMockMediaRepository() *MockMediaRepository {
	return &MockMediaRepository{Store: make(map[string]*domain.Media)}
}

func (m *MockMediaRepository) Create(ctx context.Context, media *domain.Media) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.CreateErr != nil {
		return m.CreateErr
	}
	if m.Store == nil {
		m.Store = make(map[string]*domain.Media)
	}
	cp := *media
	m.Store[media.MediaID] = &cp
	m.Created = append(m.Created, &cp)
	return nil
}

func (m *MockMediaRepository) GetByID(ctx context.Context, mediaID string) (*domain.Media, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetByIDCalls = append(m.GetByIDCalls, mediaID)
	if m.Media != nil {
		return m.Media, nil
	}
	if item, ok := m.Store[mediaID]; ok {
		return item, nil
	}
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	return nil, domain.ErrMediaNotFound
}

func (m *MockMediaRepository) UpdateStatus(ctx context.Context, mediaID string, expectedCurrent, next domain.MediaStatus) error {
	m.mu.Lock()
	m.UpdateStatuses = append(m.UpdateStatuses, MediaStatusCall{MediaID: mediaID, Current: expectedCurrent, Next: next})
	call := MediaStatusCall{MediaID: mediaID, Current: expectedCurrent, Next: next}
	hook := m.OnUpdateStatus
	err := m.UpdateStatusErr
	if err == nil && hook == nil {
		if item, ok := m.Store[mediaID]; ok {
			item.Status = next
			item.UpdatedAt = time.Now().UTC()
		}
		if m.Media != nil && m.Media.MediaID == mediaID {
			m.Media.Status = next
			m.Media.UpdatedAt = time.Now().UTC()
		}
	}
	m.mu.Unlock()
	if hook != nil {
		return hook(call)
	}
	return err
}

func (m *MockMediaRepository) ListByOwner(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Media, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListOwnerCalls = append(m.ListOwnerCalls, ListOwnerCall{OwnerID: ownerID, Limit: limit, Cursor: cursor})
	if m.ListErr != nil {
		return nil, "", m.ListErr
	}
	return m.ListResults, m.ListCursor, nil
}

func (m *MockMediaRepository) Delete(ctx context.Context, mediaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Deleted = append(m.Deleted, mediaID)
	return m.DeleteErr
}

// UploadStatusCall records an UploadRepository UpdateStatus invocation.
type UploadStatusCall struct {
	UploadID string
	Status   domain.UploadStatus
}

// MockUploadRepository is an in-memory UploadRepository.
type MockUploadRepository struct {
	mu sync.Mutex

	// Session, when set, is returned by GetByID regardless of stored items.
	Session *domain.UploadSession
	// GetErr overrides GetByID.
	GetErr error
	// CreateErr overrides Create.
	CreateErr error
	// UpdateStatusErr overrides UpdateStatus.
	UpdateStatusErr error
	// UpdatePartsErr overrides UpdateCompletedParts.
	UpdatePartsErr error
	// UpdateExpiryErr overrides UpdateExpiry.
	UpdateExpiryErr error
	// ListExpiredErr overrides ListExpired.
	ListExpiredErr error

	// ExpiredSessions returned by ListExpired.
	ExpiredSessions []*domain.UploadSession

	Store map[string]*domain.UploadSession

	Created           []*domain.UploadSession
	StatusUpdates     []UploadStatusCall
	CompletedParts    map[string][]domain.UploadPart
	ExpiryUpdates     map[string]time.Time
	ListExpiredBefore []time.Time
}

// NewMockUploadRepository returns an empty upload repository.
func NewMockUploadRepository() *MockUploadRepository {
	return &MockUploadRepository{
		Store:          make(map[string]*domain.UploadSession),
		CompletedParts: make(map[string][]domain.UploadPart),
		ExpiryUpdates:  make(map[string]time.Time),
	}
}

func (r *MockUploadRepository) Create(ctx context.Context, session *domain.UploadSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CreateErr != nil {
		return r.CreateErr
	}
	if r.Store == nil {
		r.Store = make(map[string]*domain.UploadSession)
	}
	cp := *session
	r.Store[session.UploadID] = &cp
	r.Created = append(r.Created, &cp)
	return nil
}

func (r *MockUploadRepository) GetByID(ctx context.Context, uploadID string) (*domain.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Session != nil {
		return r.Session, nil
	}
	if item, ok := r.Store[uploadID]; ok {
		return item, nil
	}
	if r.GetErr != nil {
		return nil, r.GetErr
	}
	return nil, domain.ErrUploadSessionNotFound
}

func (r *MockUploadRepository) UpdateStatus(ctx context.Context, uploadID string, status domain.UploadStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.StatusUpdates = append(r.StatusUpdates, UploadStatusCall{UploadID: uploadID, Status: status})
	if item, ok := r.Store[uploadID]; ok && r.UpdateStatusErr == nil {
		item.Status = status
	}
	return r.UpdateStatusErr
}

func (r *MockUploadRepository) UpdateCompletedParts(ctx context.Context, uploadID string, parts []domain.UploadPart) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.UpdatePartsErr != nil {
		return r.UpdatePartsErr
	}
	if r.CompletedParts == nil {
		r.CompletedParts = make(map[string][]domain.UploadPart)
	}
	r.CompletedParts[uploadID] = parts
	if item, ok := r.Store[uploadID]; ok {
		item.CompletedParts = parts
	}
	return nil
}

func (r *MockUploadRepository) UpdateExpiry(ctx context.Context, uploadID string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.UpdateExpiryErr != nil {
		return r.UpdateExpiryErr
	}
	if r.ExpiryUpdates == nil {
		r.ExpiryUpdates = make(map[string]time.Time)
	}
	r.ExpiryUpdates[uploadID] = expiresAt
	if item, ok := r.Store[uploadID]; ok {
		item.ExpiresAt = expiresAt
	}
	return nil
}

func (r *MockUploadRepository) ListExpired(ctx context.Context, before time.Time) ([]*domain.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ListExpiredBefore = append(r.ListExpiredBefore, before)
	if r.ListExpiredErr != nil {
		return nil, r.ListExpiredErr
	}
	return r.ExpiredSessions, nil
}

// MockJobRepository is an in-memory JobRepository.
type MockJobRepository struct {
	mu sync.Mutex

	Job             *domain.MediaJob
	GetErr          error
	CreateErr       error
	UpdateStatusErr error
	ListStuckErr    error

	StuckJobs []*domain.MediaJob

	Created      []*domain.MediaJob
	StatusUpdate struct {
		JobID     string
		Status    domain.JobStatus
		LastError string
		Called    bool
	}
}

func (r *MockJobRepository) Create(ctx context.Context, job *domain.MediaJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CreateErr != nil {
		return r.CreateErr
	}
	cp := *job
	r.Created = append(r.Created, &cp)
	return nil
}

func (r *MockJobRepository) GetByID(ctx context.Context, jobID string) (*domain.MediaJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.GetErr != nil {
		return nil, r.GetErr
	}
	if r.Job == nil {
		return nil, domain.ErrJobNotFound
	}
	return r.Job, nil
}

func (r *MockJobRepository) UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, lastError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.StatusUpdate = struct {
		JobID     string
		Status    domain.JobStatus
		LastError string
		Called    bool
	}{JobID: jobID, Status: status, LastError: lastError, Called: true}
	return r.UpdateStatusErr
}

func (r *MockJobRepository) ListStuck(ctx context.Context, stuckBefore time.Time) ([]*domain.MediaJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ListStuckErr != nil {
		return nil, r.ListStuckErr
	}
	return r.StuckJobs, nil
}

// MockOutboxRepository is an in-memory OutboxRepository.
type MockOutboxRepository struct {
	mu sync.Mutex

	CreateErr        error
	ListPendingErr   error
	MarkPublishedErr error
	MarkFailedErr    error

	Pending   []*domain.OutboxEvent
	Store     map[string]*domain.OutboxEvent
	Created   []*domain.OutboxEvent
	Published []string
	Failed    map[string]string
}

// NewMockOutboxRepository returns an empty outbox repository.
func NewMockOutboxRepository() *MockOutboxRepository {
	return &MockOutboxRepository{Store: make(map[string]*domain.OutboxEvent), Failed: make(map[string]string)}
}

func (r *MockOutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CreateErr != nil {
		return r.CreateErr
	}
	if r.Store == nil {
		r.Store = make(map[string]*domain.OutboxEvent)
	}
	cp := *event
	r.Store[event.EventID] = &cp
	r.Created = append(r.Created, &cp)
	return nil
}

func (r *MockOutboxRepository) ListPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ListPendingErr != nil {
		return nil, r.ListPendingErr
	}
	return r.Pending, nil
}

func (r *MockOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.MarkPublishedErr != nil {
		return r.MarkPublishedErr
	}
	r.Published = append(r.Published, eventID)
	return nil
}

func (r *MockOutboxRepository) MarkFailed(ctx context.Context, eventID string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.MarkFailedErr != nil {
		return r.MarkFailedErr
	}
	if r.Failed == nil {
		r.Failed = make(map[string]string)
	}
	r.Failed[eventID] = reason
	return nil
}

// MockQuotaRepository is an in-memory QuotaRepository.
type MockQuotaRepository struct {
	mu sync.Mutex

	Usage            *domain.QuotaUsage
	GetErr           error
	IncrementErr     error
	DecrementErr     error
	AddBytesErr      error
	SubtractBytesErr error

	Increments      []int64
	Decrements      int
	AddedBytes      []int64
	SubtractedBytes []int64
}

// NewMockQuotaRepository returns a repository with an unlimited quota.
func NewMockQuotaRepository() *MockQuotaRepository {
	return &MockQuotaRepository{
		Usage: &domain.QuotaUsage{
			QuotaBytes:    1 << 40,
			MaxConcurrent: 100,
		},
	}
}

func (r *MockQuotaRepository) GetUsage(ctx context.Context, userID string) (*domain.QuotaUsage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.GetErr != nil {
		return nil, r.GetErr
	}
	if r.Usage == nil {
		return &domain.QuotaUsage{UserID: userID, QuotaBytes: 1 << 40, MaxConcurrent: 100}, nil
	}
	cp := *r.Usage
	cp.UserID = userID
	return &cp, nil
}

func (r *MockQuotaRepository) IncrementUsage(ctx context.Context, userID string, bytes int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Increments = append(r.Increments, bytes)
	return r.IncrementErr
}

func (r *MockQuotaRepository) DecrementActiveUpload(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Decrements++
	return r.DecrementErr
}

func (r *MockQuotaRepository) AddUsedBytes(ctx context.Context, userID string, bytes int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.AddedBytes = append(r.AddedBytes, bytes)
	return r.AddBytesErr
}

func (r *MockQuotaRepository) SubtractUsedBytes(ctx context.Context, userID string, bytes int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.SubtractedBytes = append(r.SubtractedBytes, bytes)
	return r.SubtractBytesErr
}

// MockVersionRepository is an in-memory VersionRepository.
type MockVersionRepository struct {
	mu sync.Mutex

	CreateErr error
	ListErr   error

	Versions  map[string][]*domain.MediaVersion
	Created   []*domain.MediaVersion
	ListCalls []string
}

// NewMockVersionRepository returns an empty version repository.
func NewMockVersionRepository() *MockVersionRepository {
	return &MockVersionRepository{Versions: make(map[string][]*domain.MediaVersion)}
}

func (r *MockVersionRepository) Create(ctx context.Context, version *domain.MediaVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CreateErr != nil {
		return r.CreateErr
	}
	if r.Versions == nil {
		r.Versions = make(map[string][]*domain.MediaVersion)
	}
	cp := *version
	r.Versions[version.MediaID] = append(r.Versions[version.MediaID], &cp)
	r.Created = append(r.Created, &cp)
	return nil
}

func (r *MockVersionRepository) ListByMedia(ctx context.Context, mediaID string) ([]*domain.MediaVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ListCalls = append(r.ListCalls, mediaID)
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	return r.Versions[mediaID], nil
}
