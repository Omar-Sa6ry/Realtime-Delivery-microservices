package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type MockIdempotencyStore struct {
	mu           sync.RWMutex
	seen         map[string]*domain.IdempotencyRecord
	checkpoints  map[string]*domain.IngestionCheckpoint
	existsErr    error
	saveErr      error
	loadChkptErr error
	saveChkptErr error
}

func NewMockIdempotencyStore() *MockIdempotencyStore {
	return &MockIdempotencyStore{
		seen:        make(map[string]*domain.IdempotencyRecord),
		checkpoints: make(map[string]*domain.IngestionCheckpoint),
	}
}

func (m *MockIdempotencyStore) SetExistsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.existsErr = err
}

func (m *MockIdempotencyStore) SetSaveError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveErr = err
}

func (m *MockIdempotencyStore) SetLoadCheckpointError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadChkptErr = err
}

func (m *MockIdempotencyStore) SetSaveCheckpointError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveChkptErr = err
}

func (m *MockIdempotencyStore) Exists(ctx context.Context, eventID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, ok := m.seen[eventID]
	return ok, nil
}

func (m *MockIdempotencyStore) Save(ctx context.Context, record *domain.IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.seen[record.EventID] = record
	return nil
}

func (m *MockIdempotencyStore) LoadCheckpoint(ctx context.Context, topic string, partition int32) (*domain.IngestionCheckpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.loadChkptErr != nil {
		return nil, m.loadChkptErr
	}
	key := fmt.Sprintf("%s-%d", topic, partition)
	return m.checkpoints[key], nil
}

func (m *MockIdempotencyStore) SaveCheckpoint(ctx context.Context, checkpoint *domain.IngestionCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveChkptErr != nil {
		return m.saveChkptErr
	}
	key := fmt.Sprintf("%s-%d", checkpoint.Topic, checkpoint.Partition)
	m.checkpoints[key] = checkpoint
	return nil
}

func (m *MockIdempotencyStore) Close() error {
	return nil
}

type MockClickHouseWriter struct {
	mu             sync.RWMutex
	rawEvents      []*domain.RawEventLanding
	deliveryEvents []*domain.FactDeliveryEvent
	deliveryComp   []*domain.FactDeliveryCompleted
	driverAssigns  []*domain.FactDriverAssignment
	payments       []*domain.FactPaymentTransaction
	notifications  []*domain.FactNotificationEvent
	dataQuality    []*domain.DataQualityIssue
	writeErr       error
}

func NewMockClickHouseWriter() *MockClickHouseWriter {
	return &MockClickHouseWriter{}
}

func (m *MockClickHouseWriter) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

func (m *MockClickHouseWriter) WriteRawEvents(ctx context.Context, rows []*domain.RawEventLanding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.rawEvents = append(m.rawEvents, rows...)
	return nil
}

func (m *MockClickHouseWriter) WriteDeliveryEvents(ctx context.Context, rows []*domain.FactDeliveryEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.deliveryEvents = append(m.deliveryEvents, rows...)
	return nil
}

func (m *MockClickHouseWriter) WriteDeliveryCompleted(ctx context.Context, rows []*domain.FactDeliveryCompleted) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.deliveryComp = append(m.deliveryComp, rows...)
	return nil
}

func (m *MockClickHouseWriter) WriteDriverAssignments(ctx context.Context, rows []*domain.FactDriverAssignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.driverAssigns = append(m.driverAssigns, rows...)
	return nil
}

func (m *MockClickHouseWriter) WritePaymentTransactions(ctx context.Context, rows []*domain.FactPaymentTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.payments = append(m.payments, rows...)
	return nil
}

func (m *MockClickHouseWriter) WriteNotificationEvents(ctx context.Context, rows []*domain.FactNotificationEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.notifications = append(m.notifications, rows...)
	return nil
}

func (m *MockClickHouseWriter) WriteDataQualityIssues(ctx context.Context, rows []*domain.DataQualityIssue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.dataQuality = append(m.dataQuality, rows...)
	return nil
}

func (m *MockClickHouseWriter) Close() error {
	return nil
}

func (m *MockClickHouseWriter) GetRawEvents() []*domain.RawEventLanding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.RawEventLanding, len(m.rawEvents))
	copy(result, m.rawEvents)
	return result
}

func (m *MockClickHouseWriter) GetDeliveryEvents() []*domain.FactDeliveryEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.FactDeliveryEvent, len(m.deliveryEvents))
	copy(result, m.deliveryEvents)
	return result
}

func (m *MockClickHouseWriter) GetDeliveryCompleted() []*domain.FactDeliveryCompleted {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.FactDeliveryCompleted, len(m.deliveryComp))
	copy(result, m.deliveryComp)
	return result
}

func (m *MockClickHouseWriter) GetDriverAssignments() []*domain.FactDriverAssignment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.FactDriverAssignment, len(m.driverAssigns))
	copy(result, m.driverAssigns)
	return result
}

func (m *MockClickHouseWriter) GetPaymentTransactions() []*domain.FactPaymentTransaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.FactPaymentTransaction, len(m.payments))
	copy(result, m.payments)
	return result
}

func (m *MockClickHouseWriter) GetNotificationEvents() []*domain.FactNotificationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.FactNotificationEvent, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func (m *MockClickHouseWriter) GetDataQualityIssues() []*domain.DataQualityIssue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.DataQualityIssue, len(m.dataQuality))
	copy(result, m.dataQuality)
	return result
}

type MockCacheRepository struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMockCacheRepository() *MockCacheRepository {
	return &MockCacheRepository{
		data: make(map[string][]byte),
	}
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) (value []byte, hit bool, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *MockCacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockCacheRepository) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockCacheRepository) Close() error {
	return nil
}

type MockAnalyticsQueryRepository struct {
	platformOverview     *ports.PlatformOverview
	deliveryAnalytics    *ports.DeliveryAnalytics
	driverAnalytics      *ports.DriverAnalytics
	topDriversPage       *ports.DriverAnalyticsPage
	paymentAnalytics     *ports.PaymentAnalytics
	rawEventsPage        *ports.RawEventsPage
	dataQualityIssuesPage *ports.DataQualityIssuesPage
	reconciliationQueries *MockReconciliationQueries
	err                  error
}

func NewMockAnalyticsQueryRepository() *MockAnalyticsQueryRepository {
	return &MockAnalyticsQueryRepository{
		reconciliationQueries: NewMockReconciliationQueries(),
	}
}

func (m *MockAnalyticsQueryRepository) SetError(err error) {
	m.err = err
}

func (m *MockAnalyticsQueryRepository) SetPlatformOverview(o *ports.PlatformOverview) {
	m.platformOverview = o
}

func (m *MockAnalyticsQueryRepository) SetDeliveryAnalytics(a *ports.DeliveryAnalytics) {
	m.deliveryAnalytics = a
}

func (m *MockAnalyticsQueryRepository) SetDriverAnalytics(a *ports.DriverAnalytics) {
	m.driverAnalytics = a
}

func (m *MockAnalyticsQueryRepository) SetTopDriversPage(p *ports.DriverAnalyticsPage) {
	m.topDriversPage = p
}

func (m *MockAnalyticsQueryRepository) SetPaymentAnalytics(a *ports.PaymentAnalytics) {
	m.paymentAnalytics = a
}

func (m *MockAnalyticsQueryRepository) SetRawEventsPage(p *ports.RawEventsPage) {
	m.rawEventsPage = p
}

func (m *MockAnalyticsQueryRepository) SetDataQualityIssuesPage(p *ports.DataQualityIssuesPage) {
	m.dataQualityIssuesPage = p
}

func (m *MockAnalyticsQueryRepository) PlatformOverview(ctx context.Context, r ports.TimeRange) (*ports.PlatformOverview, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.platformOverview, nil
}

func (m *MockAnalyticsQueryRepository) DeliveryAnalytics(ctx context.Context, f ports.DeliveryAnalyticsFilter) (*ports.DeliveryAnalytics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.deliveryAnalytics, nil
}

func (m *MockAnalyticsQueryRepository) DriverAnalytics(ctx context.Context, f ports.DriverAnalyticsFilter) (*ports.DriverAnalytics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.driverAnalytics, nil
}

func (m *MockAnalyticsQueryRepository) TopDrivers(ctx context.Context, r ports.TimeRange, limit int) (*ports.DriverAnalyticsPage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.topDriversPage, nil
}

func (m *MockAnalyticsQueryRepository) PaymentAnalytics(ctx context.Context, f ports.PaymentAnalyticsFilter) (*ports.PaymentAnalytics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.paymentAnalytics, nil
}

func (m *MockAnalyticsQueryRepository) RawEvents(ctx context.Context, page, limit int, eventType string, from, to *time.Time) (*ports.RawEventsPage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rawEventsPage, nil
}

func (m *MockAnalyticsQueryRepository) DataQualityIssues(ctx context.Context, page, limit int, severity string) (*ports.DataQualityIssuesPage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.dataQualityIssuesPage, nil
}

func (m *MockAnalyticsQueryRepository) Close() error {
	return nil
}

func (m *MockAnalyticsQueryRepository) ReconciliationQueries() ports.ReconciliationQueries {
	return m.reconciliationQueries
}

type MockReconciliationQueries struct {
	maxRawOccurredAt     time.Time
	countRawEvents       int64
	countFactDelivery    int64
	countDataQuality     int64
	distinctRawEventTypes []string
	openIssuesBySeverity map[string]int64
	err                  error
}

func NewMockReconciliationQueries() *MockReconciliationQueries {
	return &MockReconciliationQueries{
		openIssuesBySeverity: make(map[string]int64),
	}
}

func (m *MockReconciliationQueries) SetError(err error) {
	m.err = err
}

func (m *MockReconciliationQueries) SetMaxRawOccurredAt(t time.Time) {
	m.maxRawOccurredAt = t
}

func (m *MockReconciliationQueries) SetCountRawEvents(n int64) {
	m.countRawEvents = n
}

func (m *MockReconciliationQueries) SetCountFactDeliveryEvents(n int64) {
	m.countFactDelivery = n
}

func (m *MockReconciliationQueries) SetCountDataQualityByType(n int64) {
	m.countDataQuality = n
}

func (m *MockReconciliationQueries) SetDistinctRawEventTypes(types []string) {
	m.distinctRawEventTypes = types
}

func (m *MockReconciliationQueries) SetOpenIssuesBySeverity(issues map[string]int64) {
	m.openIssuesBySeverity = issues
}

func (m *MockReconciliationQueries) MaxRawOccurredAt(ctx context.Context) (time.Time, error) {
	if m.err != nil {
		return time.Time{}, m.err
	}
	return m.maxRawOccurredAt, nil
}

func (m *MockReconciliationQueries) CountRawEvents(ctx context.Context, from, to time.Time) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.countRawEvents, nil
}

func (m *MockReconciliationQueries) CountFactDeliveryEvents(ctx context.Context, from, to time.Time) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.countFactDelivery, nil
}

func (m *MockReconciliationQueries) CountDataQualityByType(ctx context.Context, issueType string, from, to time.Time) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.countDataQuality, nil
}

func (m *MockReconciliationQueries) DistinctRawEventTypes(ctx context.Context, from, to time.Time) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.distinctRawEventTypes, nil
}

func (m *MockReconciliationQueries) CountOpenIssuesBySeverity(ctx context.Context) (map[string]int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.openIssuesBySeverity, nil
}