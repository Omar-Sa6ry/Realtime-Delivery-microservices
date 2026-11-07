package graphql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ---------------------------------------------------------------------------
// Test environment: wires the real NewHandler + NewRootResolver on top of the
// in-memory doubles from internal/mocks, exactly the way production does.
// ---------------------------------------------------------------------------

type env struct {
	t *testing.T

	mockDrivers     *mocks.MockDriverRepository
	mockAssignments *mocks.MockAssignmentRepository
	reviewRepo      *mocks.MockReviewRepository
	publisher       *mocks.MockEventPublisher
	lockManager     *mocks.MockLockManager
	locationStore   *mocks.MockLocationStore
	mockClient      *mocks.MockDeliveryServiceClient

	driverRepo     ports.DriverRepository
	assignmentRepo ports.AssignmentRepository
	client         ports.DeliveryServiceClient

	noDispatch        bool
	noLoaders         bool
	rootPublisherNil  bool

	dispatch *services.DispatchService
	loaders  *graphql.Loaders
	root     *graphql.RootResolver
	handler  http.HandlerFunc
}

type envOption func(*env)

func withDriverRepo(r ports.DriverRepository) envOption {
	return func(e *env) { e.driverRepo = r }
}

// withAssignmentRepo replaces the assignment repository after defaults exist.
func withAssignmentRepo(r ports.AssignmentRepository) envOption {
	return func(e *env) { e.assignmentRepo = r }
}

func withDeliveryClient(c ports.DeliveryServiceClient) envOption {
	return func(e *env) { e.client = c }
}

func withoutDeliveryClient() envOption {
	return func(e *env) { e.client = nil }
}

func withoutDispatch() envOption {
	return func(e *env) { e.noDispatch = true }
}

func withoutLoaders() envOption {
	return func(e *env) { e.noLoaders = true }
}

func withoutRootPublisher() envOption {
	return func(e *env) { e.rootPublisherNil = true }
}

// withDriverSaveError keeps lookups working but makes every Save fail.
func withDriverSaveError(err error) envOption {
	return func(e *env) {
		e.driverRepo = &saveErrorDriverRepo{DriverRepository: e.mockDrivers, err: err}
	}
}

// withBatchLoadError simulates a repository that fails only on the batched
// FindByIDs call used by the dataloader.
func withBatchLoadError(err error) envOption {
	return func(e *env) {
		e.driverRepo = &batchErrorDriverRepo{DriverRepository: e.mockDrivers, err: err}
	}
}

// withGhostDriver serves a candidate whose driver record cannot be loaded.
func withGhostDriver(ghost *domain.Driver) envOption {
	return func(e *env) {
		e.driverRepo = &ghostDriverRepo{DriverRepository: e.mockDrivers, ghost: ghost}
	}
}

// withAssignmentLookupErrors makes FindActiveByDriver / FindByDeliveryID fail.
func withAssignmentLookupErrors(activeErr, deliveryErr error) envOption {
	return func(e *env) {
		e.assignmentRepo = &scriptedAssignmentRepo{
			AssignmentRepository: e.mockAssignments,
			activeErr:            activeErr,
			deliveryErr:          deliveryErr,
		}
	}
}

// withFlakyAssignmentSave fails Save starting from the given 1-based call.
func withFlakyAssignmentSave(failFrom int) envOption {
	return func(e *env) {
		e.assignmentRepo = &flakySaveAssignmentRepo{
			AssignmentRepository: e.mockAssignments,
			failFrom:             failFrom,
		}
	}
}

// withOpenDeliveries swaps in a delivery client with scripted open deliveries.
func withOpenDeliveries(items []*ports.OpenDeliveryInfo, total int32, err error) envOption {
	return func(e *env) {
		e.client = &fakeOpenDeliveriesClient{items: items, total: total, err: err}
	}
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	e := &env{
		t:               t,
		mockDrivers:     mocks.NewMockDriverRepository(),
		mockAssignments: mocks.NewMockAssignmentRepository(),
		reviewRepo:      mocks.NewMockReviewRepository(),
		publisher:       mocks.NewMockEventPublisher(),
		lockManager:     mocks.NewMockLockManager(),
		locationStore:   mocks.NewMockLocationStore(),
		mockClient:      mocks.NewMockDeliveryServiceClient(),
	}
	e.driverRepo = e.mockDrivers
	e.assignmentRepo = e.mockAssignments
	e.client = e.mockClient

	for _, opt := range opts {
		opt(e)
	}

	if !e.noDispatch {
		e.dispatch = services.NewDispatchService(
			e.driverRepo, e.assignmentRepo, e.locationStore, e.lockManager, e.publisher, nil, nil,
		)
	}
	if !e.noLoaders {
		e.loaders = graphql.NewLoaders(e.driverRepo)
	}

	var rootPublisher ports.EventPublisher = e.publisher
	if e.rootPublisherNil {
		rootPublisher = nil
	}

	e.root = graphql.NewRootResolver(
		e.driverRepo,
		e.assignmentRepo,
		e.dispatch,
		commands.NewRegisterDriverHandler(e.driverRepo, e.publisher, nil),
		commands.NewBlockDriverHandler(e.driverRepo, e.publisher),
		commands.NewUnblockDriverHandler(e.driverRepo, e.publisher),
		commands.NewRateDriverHandler(e.driverRepo, e.reviewRepo, e.client, e.assignmentRepo),
		queries.NewGetDriverReviewsHandler(e.reviewRepo, e.driverRepo),
		rootPublisher,
		e.client,
	)
	e.handler = graphql.NewHandler(e.root, e.loaders)
	return e
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

type result struct {
	status int
	header http.Header
	body   string
	json   map[string]interface{}
}

func (e *env) request(method, target, body string, headers map[string]string) *result {
	e.t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.handler(rec, req)
	res := &result{status: rec.Code, header: rec.Header(), body: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &res.json)
	return res
}

func (e *env) gql(query string, vars map[string]interface{}, headers map[string]string) *result {
	e.t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": vars,
	})
	if err != nil {
		e.t.Fatalf("failed to marshal GraphQL request: %v", err)
	}
	return e.request(http.MethodPost, "/graphql", string(payload), headers)
}

func userHeaders(userID, role string) map[string]string {
	h := map[string]string{}
	if userID != "" {
		h["x-user-id"] = userID
	}
	if role != "" {
		h["x-user-role"] = role
	}
	return h
}

// ---------------------------------------------------------------------------
// JSON assertions (plain t.Errorf / t.Fatalf, no testify)
// ---------------------------------------------------------------------------

// walk descends into the decoded JSON. Each element of path may be a single
// key or a dotted path ("data.driver.success"), so call sites can pass either
// style; every dot-separated segment is resolved in turn.
func walk(root interface{}, path []string) (interface{}, bool) {
	cur := root
	for _, p := range path {
		for _, seg := range strings.Split(p, ".") {
			if seg == "" {
				continue
			}
			switch c := cur.(type) {
			case map[string]interface{}:
				next, ok := c[seg]
				if !ok {
					return nil, false
				}
				cur = next
			case []interface{}:
				idx, err := strconv.Atoi(seg)
				if err != nil || idx < 0 || idx >= len(c) {
					return nil, false
				}
				cur = c[idx]
			default:
				return nil, false
			}
		}
	}
	return cur, true
}

func wantStr(t *testing.T, root interface{}, want string, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	got, ok := v.(string)
	if !ok {
		t.Fatalf("path %v: got %T (%v), want string", path, v, v)
	}
	if got != want {
		t.Errorf("path %v: got %q, want %q", path, got, want)
	}
}

func wantContains(t *testing.T, root interface{}, substr string, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	got, ok := v.(string)
	if !ok {
		t.Fatalf("path %v: got %T (%v), want string", path, v, v)
	}
	if !strings.Contains(got, substr) {
		t.Errorf("path %v: got %q, want it to contain %q", path, got, substr)
	}
}

func wantNum(t *testing.T, root interface{}, want float64, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	got, ok := v.(float64)
	if !ok {
		t.Fatalf("path %v: got %T (%v), want number", path, v, v)
	}
	if got != want {
		t.Errorf("path %v: got %v, want %v", path, got, want)
	}
}

func wantBool(t *testing.T, root interface{}, want bool, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	got, ok := v.(bool)
	if !ok {
		t.Fatalf("path %v: got %T (%v), want bool", path, v, v)
	}
	if got != want {
		t.Errorf("path %v: got %v, want %v", path, got, want)
	}
}

func wantNull(t *testing.T, root interface{}, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	if v != nil {
		t.Errorf("path %v: got %v, want null", path, v)
	}
}

func wantNotNull(t *testing.T, root interface{}, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	if v == nil {
		t.Errorf("path %v: got null, want a value", path)
	}
}

func wantListLen(t *testing.T, root interface{}, want int, path ...string) {
	t.Helper()
	v, ok := walk(root, path)
	if !ok {
		t.Fatalf("path %v not found in %v", path, root)
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("path %v: got %T, want list", path, v)
	}
	if len(arr) != want {
		t.Errorf("path %v: got %d items, want %d", path, len(arr), want)
	}
}

func wantNoErrors(t *testing.T, res *result) {
	t.Helper()
	if res.json == nil {
		t.Fatalf("response is not JSON: %s", res.body)
	}
	if errs, ok := res.json["errors"]; ok && errs != nil {
		t.Errorf("unexpected GraphQL errors: %v", errs)
	}
}

func wantSomeErrors(t *testing.T, res *result) {
	t.Helper()
	if res.json == nil {
		t.Fatalf("response is not JSON: %s", res.body)
	}
	errs, ok := res.json["errors"]
	if !ok || errs == nil {
		t.Fatalf("expected GraphQL errors, got none: %s", res.body)
	}
	arr, ok := errs.([]interface{})
	if !ok || len(arr) == 0 {
		t.Fatalf("expected a non-empty errors array, got: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Seed data
// ---------------------------------------------------------------------------

var (
	testCreatedAt  = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	testUpdatedAt  = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	testOfferedAt  = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	testExpiresAt  = time.Date(2026, 3, 4, 5, 6, 37, 0, time.UTC)
	testAcceptedAt = time.Date(2026, 3, 4, 6, 0, 0, 0, time.UTC)
	testRejectedAt = time.Date(2026, 3, 4, 7, 0, 0, 0, time.UTC)
	testCompletedAt = time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)
	testReviewAt   = time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
)

const (
	createdAtRFC  = "2026-01-02T03:04:05Z"
	updatedAtRFC  = "2026-02-03T04:05:06Z"
	offeredAtRFC  = "2026-03-04T05:06:07Z"
	expiresAtRFC  = "2026-03-04T05:06:37Z"
	acceptedAtRFC = "2026-03-04T06:00:00Z"
	rejectedAtRFC = "2026-03-04T07:00:00Z"
	completedAtRFC = "2026-03-04T08:00:00Z"
	reviewAtRFC   = "2026-04-05T06:07:08Z"
)

// driverFields selects every Driver field, including the nullable optionals so
// both nil and non-nil branches of DriverResolver are exercised.
const driverFields = `id userId status vehicleType plateNumber capacityKg capabilities serviceArea rating isActive isBlocked createdAt updatedAt user { id }`

func newRichDriver(id, userID string) *domain.Driver {
	return &domain.Driver{
		ID:           id,
		UserID:       userID,
		Status:       domain.DriverStatusOffline,
		IsActive:     true,
		Vehicle:      domain.VehicleInfo{Type: domain.VehicleTypeCar, PlateNumber: "PLATE-" + id, CapacityKg: 75},
		Capabilities: []string{"STANDARD", "FRAGILE"},
		ServiceArea:  "Cairo",
		Rating:       4.5,
		RatingCount:  3,
		CreatedAt:    testCreatedAt,
		UpdatedAt:    testUpdatedAt,
	}
}

// newBareDriver leaves every optional field at its zero value.
func newBareDriver(id string) *domain.Driver {
	return &domain.Driver{ID: id, Status: domain.DriverStatusOffline}
}

func newAvailableDriver(id, userID string) *domain.Driver {
	d := newRichDriver(id, userID)
	d.Status = domain.DriverStatusAvailable
	return d
}

func newOfferedAssignment(id, driverID, deliveryID string) *domain.Assignment {
	return &domain.Assignment{
		ID:            id,
		DriverID:      driverID,
		DeliveryID:    deliveryID,
		Status:        domain.AssignmentStatusOffered,
		AttemptNumber: 2,
		OfferedAt:     testOfferedAt,
		ExpiresAt:     testExpiresAt,
		CreatedAt:     testCreatedAt,
		UpdatedAt:     testUpdatedAt,
	}
}

// newStampedAssignment additionally fills the optional timestamp pointers.
func newStampedAssignment(id, driverID, deliveryID string) *domain.Assignment {
	a := newOfferedAssignment(id, driverID, deliveryID)
	acc, rej, comp := testAcceptedAt, testRejectedAt, testCompletedAt
	a.AcceptedAt, a.RejectedAt, a.CompletedAt = &acc, &rej, &comp
	return a
}

func newReview(id, driverID, userID, deliveryID string, rating float64, comment string) *domain.Review {
	return &domain.Review{
		ID:         id,
		DriverID:   driverID,
		UserID:     userID,
		DeliveryID: deliveryID,
		Rating:     rating,
		Comment:    comment,
		CreatedAt:  testReviewAt,
	}
}

// ---------------------------------------------------------------------------
// Hand-written fakes (extra doubles live here, never in internal/mocks)
// ---------------------------------------------------------------------------

// saveErrorDriverRepo keeps lookups working but fails Save.
type saveErrorDriverRepo struct {
	ports.DriverRepository
	err error
}

func (r *saveErrorDriverRepo) Save(ctx context.Context, d *domain.Driver) error {
	return r.err
}

// batchErrorDriverRepo fails only the batched FindByIDs call used by loaders.
type batchErrorDriverRepo struct {
	ports.DriverRepository
	err error
}

func (r *batchErrorDriverRepo) FindByIDs(ctx context.Context, ids []string) ([]*domain.Driver, error) {
	return nil, r.err
}

// ghostDriverRepo advertises a candidate that cannot be loaded afterwards.
type ghostDriverRepo struct {
	ports.DriverRepository
	ghost *domain.Driver
}

func (r *ghostDriverRepo) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vt domain.VehicleType) ([]*domain.Driver, error) {
	return []*domain.Driver{r.ghost}, nil
}

func (r *ghostDriverRepo) FindByID(ctx context.Context, id string) (*domain.Driver, error) {
	return nil, nil
}

// scriptedAssignmentRepo injects lookup errors on top of the mock.
type scriptedAssignmentRepo struct {
	ports.AssignmentRepository
	activeErr   error
	deliveryErr error
}

func (r *scriptedAssignmentRepo) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	if r.activeErr != nil {
		return nil, r.activeErr
	}
	return r.AssignmentRepository.FindActiveByDriver(ctx, driverID)
}

func (r *scriptedAssignmentRepo) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Assignment, error) {
	if r.deliveryErr != nil {
		return nil, r.deliveryErr
	}
	return r.AssignmentRepository.FindByDeliveryID(ctx, deliveryID)
}

// flakySaveAssignmentRepo fails Save starting from a given call number.
type flakySaveAssignmentRepo struct {
	ports.AssignmentRepository
	failFrom int
	calls    int
}

func (r *flakySaveAssignmentRepo) Save(ctx context.Context, a *domain.Assignment) error {
	r.calls++
	if r.calls >= r.failFrom {
		return errors.New("assignment save failed")
	}
	return r.AssignmentRepository.Save(ctx, a)
}

// fakeOpenDeliveriesClient scripts GetOpenDeliveries.
type fakeOpenDeliveriesClient struct {
	items []*ports.OpenDeliveryInfo
	total int32
	err   error
}

func (f *fakeOpenDeliveriesClient) GetDelivery(ctx context.Context, deliveryID string) (*ports.DeliveryInfo, error) {
	return nil, nil
}

func (f *fakeOpenDeliveriesClient) GetOpenDeliveries(ctx context.Context, page, limit int32) ([]*ports.OpenDeliveryInfo, int32, error) {
	return f.items, f.total, f.err
}
