package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// Minimal types for mocking gRPC clients
type UserInfo struct {
	ID   string
	Role string
}

type UpdateUserRoleResponse struct {
	Success bool
}

type MockDriverRepository struct {
	mu         sync.RWMutex
	drivers    map[string]*domain.Driver
	byUserID   map[string]*domain.Driver
	saveError  error
}

func NewMockDriverRepository() *MockDriverRepository {
	return &MockDriverRepository{
		drivers:  make(map[string]*domain.Driver),
		byUserID: make(map[string]*domain.Driver),
	}
}

func (m *MockDriverRepository) FindByID(ctx context.Context, id string) (*domain.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	driver, ok := m.drivers[id]
	if !ok {
		return nil, nil
	}
	return driver, nil
}

func (m *MockDriverRepository) FindByIDs(ctx context.Context, ids []string) ([]*domain.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	var result []*domain.Driver
	for _, id := range ids {
		if d, ok := m.drivers[id]; ok {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MockDriverRepository) FindByUserID(ctx context.Context, userID string) (*domain.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	driver, ok := m.byUserID[userID]
	if !ok {
		return nil, nil
	}
	return driver, nil
}

func (m *MockDriverRepository) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType domain.VehicleType) ([]*domain.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	var result []*domain.Driver
	for _, d := range m.drivers {
		if d.Status == domain.DriverStatusAvailable && d.IsActive && !d.IsBlocked {
			if vehicleType == "" || d.Vehicle.Type == vehicleType {
				result = append(result, d)
			}
		}
	}
	return result, nil
}

func (m *MockDriverRepository) FindByStatus(ctx context.Context, status string) ([]*domain.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	var result []*domain.Driver
	for _, d := range m.drivers {
		if string(d.Status) == status {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MockDriverRepository) Save(ctx context.Context, driver *domain.Driver) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveError != nil {
		return m.saveError
	}
	m.drivers[driver.ID] = driver
	m.byUserID[driver.UserID] = driver
	return nil
}

func (m *MockDriverRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveError = err
}

func (m *MockDriverRepository) UpdateRating(ctx context.Context, driverID string, averageRating float64, totalReviews int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	driver, ok := m.drivers[driverID]
	if !ok {
		return domain.ErrDriverNotFound
	}
	driver.Rating = averageRating
	driver.RatingCount = totalReviews
	driver.UpdatedAt = time.Now()
	return nil
}

func (m *MockDriverRepository) SetDriver(driver *domain.Driver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drivers[driver.ID] = driver
	m.byUserID[driver.UserID] = driver
}

type MockAssignmentRepository struct {
	mu           sync.RWMutex
	assignments  map[string]*domain.Assignment
	byDriverID   map[string]*domain.Assignment
	byDeliveryID map[string]*domain.Assignment
	saveError    error
}

func NewMockAssignmentRepository() *MockAssignmentRepository {
	return &MockAssignmentRepository{
		assignments:  make(map[string]*domain.Assignment),
		byDriverID:   make(map[string]*domain.Assignment),
		byDeliveryID: make(map[string]*domain.Assignment),
	}
}

func (m *MockAssignmentRepository) FindByID(ctx context.Context, id string) (*domain.Assignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.saveError != nil {
		return nil, m.saveError
	}
	assignment, ok := m.assignments[id]
	if !ok {
		return nil, nil
	}
	return assignment, nil
}

func (m *MockAssignmentRepository) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	assignment, ok := m.byDriverID[driverID]
	if !ok {
		return nil, nil
	}
	if assignment.Status == domain.AssignmentStatusOffered ||
		assignment.Status == domain.AssignmentStatusAccepted ||
		assignment.Status == domain.AssignmentStatusActive {
		return assignment, nil
	}
	return nil, nil
}

func (m *MockAssignmentRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Assignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	assignment, ok := m.byDeliveryID[deliveryID]
	if !ok {
		return nil, nil
	}
	return assignment, nil
}

func (m *MockAssignmentRepository) Save(ctx context.Context, assignment *domain.Assignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveError != nil {
		return m.saveError
	}
	m.assignments[assignment.ID] = assignment
	m.byDriverID[assignment.DriverID] = assignment
	m.byDeliveryID[assignment.DeliveryID] = assignment
	return nil
}

func (m *MockAssignmentRepository) UpdateStatus(ctx context.Context, assignmentID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveError != nil {
		return m.saveError
	}
	assignment, ok := m.assignments[assignmentID]
	if !ok {
		return domain.ErrAssignmentNotFound
	}
	assignment.Status = domain.AssignmentStatus(status)
	assignment.UpdatedAt = time.Now()
	return nil
}

func (m *MockAssignmentRepository) ExpireOffers(ctx context.Context, olderThan time.Duration) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var expiredIDs []string
	cutoff := time.Now().Add(-olderThan)
	for id, a := range m.assignments {
		if a.Status == domain.AssignmentStatusOffered && a.OfferedAt.Before(cutoff) {
			a.Status = domain.AssignmentStatusExpired
			a.UpdatedAt = time.Now()
			expiredIDs = append(expiredIDs, id)
		}
	}
	return expiredIDs, nil
}

func (m *MockAssignmentRepository) FindRejectedDriverIDsByDelivery(ctx context.Context, deliveryID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var driverIDs []string
	for _, a := range m.assignments {
		if a.DeliveryID == deliveryID && a.Status == domain.AssignmentStatusRejected {
			driverIDs = append(driverIDs, a.DriverID)
		}
	}
	return driverIDs, nil
}

func (m *MockAssignmentRepository) CountByDelivery(ctx context.Context, deliveryID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, a := range m.assignments {
		if a.DeliveryID == deliveryID {
			count++
		}
	}
	return count, nil
}

func (m *MockAssignmentRepository) FindRejectedByDelivery(ctx context.Context, deliveryID string) ([]*domain.Assignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rejected []*domain.Assignment
	for _, a := range m.assignments {
		if a.DeliveryID == deliveryID && a.Status == domain.AssignmentStatusRejected {
			rejected = append(rejected, a)
		}
	}
	return rejected, nil
}

func (m *MockAssignmentRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveError = err
}

func (m *MockAssignmentRepository) SetAssignment(assignment *domain.Assignment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignments[assignment.ID] = assignment
	m.byDriverID[assignment.DriverID] = assignment
	m.byDeliveryID[assignment.DeliveryID] = assignment
}

type MockReviewRepository struct {
	mu       sync.RWMutex
	reviews  map[string]*domain.Review
	byDriver map[string][]*domain.Review
	byDelivery map[string]*domain.Review
	saveError error
}

func NewMockReviewRepository() *MockReviewRepository {
	return &MockReviewRepository{
		reviews:    make(map[string]*domain.Review),
		byDriver:   make(map[string][]*domain.Review),
		byDelivery: make(map[string]*domain.Review),
	}
}

func (m *MockReviewRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveError = err
}

func (m *MockReviewRepository) Create(ctx context.Context, review *domain.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveError != nil {
		return m.saveError
	}
	m.reviews[review.ID] = review
	m.byDriver[review.DriverID] = append(m.byDriver[review.DriverID], review)
	m.byDelivery[review.DeliveryID] = review
	return nil
}

func (m *MockReviewRepository) FindByDriverID(ctx context.Context, driverID string, skip, limit int) ([]*domain.Review, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reviews := m.byDriver[driverID]
	total := int64(len(reviews))
	start := skip
	if start > len(reviews) {
		start = len(reviews)
	}
	end := start + limit
	if end > len(reviews) {
		end = len(reviews)
	}
	return reviews[start:end], total, nil
}

func (m *MockReviewRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	review, ok := m.byDelivery[deliveryID]
	if !ok {
		return nil, nil
	}
	return review, nil
}

func (m *MockReviewRepository) CalculateAverageRating(ctx context.Context, driverID string) (float64, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reviews := m.byDriver[driverID]
	if len(reviews) == 0 {
		return 0, 0, nil
	}
	var sum float64
	for _, r := range reviews {
		sum += r.Rating
	}
	return sum / float64(len(reviews)), int64(len(reviews)), nil
}

func (m *MockReviewRepository) SetReview(review *domain.Review) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reviews[review.ID] = review
	m.byDriver[review.DriverID] = append(m.byDriver[review.DriverID], review)
	m.byDelivery[review.DeliveryID] = review
}

type MockEventPublisher struct {
	mu      sync.Mutex
	events  []string
	errors  map[string]error
}

func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{
		events: make([]string, 0),
		errors: make(map[string]error),
	}
}

func (m *MockEventPublisher) PublishDriverCreated(ctx context.Context, driverID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "DriverCreated:"+driverID)
	if err, ok := m.errors["DriverCreated"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishDriverUpdated(ctx context.Context, driverID string, vehicleType domain.VehicleType, capabilities []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "DriverUpdated:"+driverID)
	if err, ok := m.errors["DriverUpdated"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishDriverDeleted(ctx context.Context, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "DriverDeleted:"+driverID)
	if err, ok := m.errors["DriverDeleted"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishAssignmentOffered(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "AssignmentOffered:"+assignmentID)
	if err, ok := m.errors["AssignmentOffered"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishAssignmentAccepted(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "AssignmentAccepted:"+assignmentID)
	if err, ok := m.errors["AssignmentAccepted"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishAssignmentRejected(ctx context.Context, assignmentID, deliveryID, driverID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "AssignmentRejected:"+assignmentID)
	if err, ok := m.errors["AssignmentRejected"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishAssignmentExpired(ctx context.Context, assignmentID, deliveryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "AssignmentExpired:"+assignmentID)
	if err, ok := m.errors["AssignmentExpired"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishAssignmentReleased(ctx context.Context, assignmentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "AssignmentReleased:"+assignmentID)
	if err, ok := m.errors["AssignmentReleased"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishDriverAvailable(ctx context.Context, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "DriverAvailable:"+driverID)
	if err, ok := m.errors["DriverAvailable"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishDriverUnavailable(ctx context.Context, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "DriverUnavailable:"+driverID)
	if err, ok := m.errors["DriverUnavailable"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishLocationUpdated(ctx context.Context, driverID, latitude, longitude string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "LocationUpdated:"+driverID)
	if err, ok := m.errors["LocationUpdated"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishNoDriverAvailable(ctx context.Context, deliveryID, customerID string, attemptNumber int, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "NoDriverAvailable:"+deliveryID)
	if err, ok := m.errors["NoDriverAvailable"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) PublishSearchRetry(ctx context.Context, deliveryID, customerID string, attemptNumber int, retryInterval int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "SearchRetry:"+deliveryID)
	if err, ok := m.errors["SearchRetry"]; ok {
		return err
	}
	return nil
}

func (m *MockEventPublisher) SetError(eventType string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[eventType] = err
}

func (m *MockEventPublisher) GetEvents() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := make([]string, len(m.events))
	copy(events, m.events)
	return events
}

func (m *MockEventPublisher) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = make([]string, 0)
}

type MockLockManager struct {
	mu     sync.Mutex
	locks  map[string]string
	tokens map[string]string
}

func NewMockLockManager() *MockLockManager {
	return &MockLockManager{
		locks:  make(map[string]string),
		tokens: make(map[string]string),
	}
}

func (m *MockLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.locks[resource]; ok {
		return false, nil
	}
	token := "token-" + resource
	m.locks[resource] = token
	m.tokens[resource] = token
	return true, nil
}

func (m *MockLockManager) Release(ctx context.Context, resource string, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks[resource] == token {
		delete(m.locks, resource)
		delete(m.tokens, resource)
		return nil
	}
	return nil
}

func (m *MockLockManager) IsLocked(resource string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.locks[resource]
	return ok
}

type MockDeliveryServiceClient struct {
	mu         sync.Mutex
	deliveries map[string]*ports.DeliveryInfo
	errors     map[string]error
}

func NewMockDeliveryServiceClient() *MockDeliveryServiceClient {
	return &MockDeliveryServiceClient{
		deliveries: make(map[string]*ports.DeliveryInfo),
		errors:     make(map[string]error),
	}
}

func (m *MockDeliveryServiceClient) GetDelivery(ctx context.Context, deliveryID string) (*ports.DeliveryInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors[deliveryID]; ok {
		return nil, err
	}
	delivery, ok := m.deliveries[deliveryID]
	if !ok {
		return nil, nil
	}
	return delivery, nil
}

func (m *MockDeliveryServiceClient) GetOpenDeliveries(ctx context.Context, page, limit int32) ([]*ports.OpenDeliveryInfo, int32, error) {
	return nil, 0, nil
}

func (m *MockDeliveryServiceClient) SetDelivery(delivery *ports.DeliveryInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliveries[delivery.DeliveryID] = delivery
}

func (m *MockDeliveryServiceClient) SetError(deliveryID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[deliveryID] = err
}

type MockUserServiceClient struct {
	mu    sync.Mutex
	users map[string]*UserInfo
	errors map[string]error
}

func NewMockUserServiceClient() *MockUserServiceClient {
	return &MockUserServiceClient{
		users:  make(map[string]*UserInfo),
		errors: make(map[string]error),
	}
}

func (m *MockUserServiceClient) GetUser(ctx context.Context, userID string) (*UserInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors[userID]; ok {
		return nil, err
	}
	user, ok := m.users[userID]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserServiceClient) UpdateUserRole(ctx context.Context, userID, role string) (*UpdateUserRoleResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors["update_"+userID]; ok {
		return &UpdateUserRoleResponse{Success: false}, err
	}
	user, ok := m.users[userID]
	if !ok {
		user = &UserInfo{ID: userID, Role: "CUSTOMER"}
		m.users[userID] = user
	}
	user.Role = role
	return &UpdateUserRoleResponse{Success: true}, nil
}

func (m *MockUserServiceClient) SetUser(user *UserInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
}

func (m *MockUserServiceClient) SetError(userID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[userID] = err
}

type MockIdempotencyStore struct {
	mu     sync.Mutex
	keys   map[string][]byte
	errors map[string]error
}

func NewMockIdempotencyStore() *MockIdempotencyStore {
	return &MockIdempotencyStore{
		keys:   make(map[string][]byte),
		errors: make(map[string]error),
	}
}

func (m *MockIdempotencyStore) CheckAndStore(ctx context.Context, key string, result []byte, ttl int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors[key]; ok {
		return false, err
	}
	if _, exists := m.keys[key]; exists {
		return false, nil
	}
	m.keys[key] = result
	return true, nil
}

func (m *MockIdempotencyStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors[key]; ok {
		return false, err
	}
	_, exists := m.keys[key]
	return exists, nil
}

func (m *MockIdempotencyStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errors[key]; ok {
		return err
	}
	delete(m.keys, key)
	return nil
}

type MockDispatchAttemptRepository struct {
	mu      sync.Mutex
	attempts map[string][]*domain.DispatchAttempt
}

func NewMockDispatchAttemptRepository() *MockDispatchAttemptRepository {
	return &MockDispatchAttemptRepository{
		attempts: make(map[string][]*domain.DispatchAttempt),
	}
}

func (m *MockDispatchAttemptRepository) Save(ctx context.Context, attempt *domain.DispatchAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[attempt.DeliveryID] = append(m.attempts[attempt.DeliveryID], attempt)
	return nil
}

func (m *MockDispatchAttemptRepository) FindByDeliveryID(ctx context.Context, deliveryID string) ([]*domain.DispatchAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempts, ok := m.attempts[deliveryID]
	if !ok {
		return []*domain.DispatchAttempt{}, nil
	}
	return attempts, nil
}

func (m *MockDispatchAttemptRepository) FindByDriverID(ctx context.Context, driverID string) ([]*domain.DispatchAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.DispatchAttempt
	for _, attempts := range m.attempts {
		for _, a := range attempts {
			if a.DriverID == driverID {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

func (m *MockDispatchAttemptRepository) CountByDeliveryID(ctx context.Context, deliveryID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempts, ok := m.attempts[deliveryID]
	if !ok {
		return 0, nil
	}
	return int64(len(attempts)), nil
}

func (m *MockDispatchAttemptRepository) EnsureIndex(ctx context.Context) error {
	return nil
}

type MockLocationStore struct {
	mu       sync.Mutex
	locations map[string]ports.GeoDriverResult
}

func NewMockLocationStore() *MockLocationStore {
	return &MockLocationStore{
		locations: make(map[string]ports.GeoDriverResult),
	}
}

func (m *MockLocationStore) GEOAdd(ctx context.Context, longitude, latitude, driverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Convert string to float64 for storage
	lon := 0.0
	lat := 0.0
	// Simple conversion - in real code would use strconv.ParseFloat
	m.locations[driverID] = ports.GeoDriverResult{
		DriverID:  driverID,
		Longitude: lon,
		Latitude:  lat,
	}
	return nil
}

func (m *MockLocationStore) GEOSearch(ctx context.Context, longitude, latitude, radiusKm float64) error {
	return nil
}

func (m *MockLocationStore) GEOSearchDrivers(ctx context.Context, longitude, latitude, radiusKm float64) ([]ports.GeoDriverResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []ports.GeoDriverResult
	for _, loc := range m.locations {
		results = append(results, ports.GeoDriverResult{
			DriverID:       loc.DriverID,
			DistanceMeters: loc.DistanceMeters,
			Longitude:      loc.Longitude,
			Latitude:       loc.Latitude,
		})
	}
	return results, nil
}

func (m *MockLocationStore) NearbyDriversSorted(ctx context.Context, longitude, latitude, radiusKm float64) ([]ports.GeoDriverResult, error) {
	return m.GEOSearchDrivers(ctx, longitude, latitude, radiusKm)
}

func (m *MockLocationStore) GetDriverLocation(ctx context.Context, driverID string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.locations[driverID]
	if !ok {
		return "", "", nil
	}
	// Convert float64 to string for return
	return "", "", nil
}

func (m *MockLocationStore) SetLocation(driverID string, longitude, latitude float64, distance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locations[driverID] = ports.GeoDriverResult{
		DriverID:       driverID,
		DistanceMeters: distance,
		Longitude:      longitude,
		Latitude:       latitude,
	}
}