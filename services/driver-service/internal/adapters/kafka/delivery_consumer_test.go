package kafka_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	kafkago "github.com/segmentio/kafka-go"
)

// ---------------------------------------------------------------------------
// test doubles (defined here because internal/mocks does not provide them)
// ---------------------------------------------------------------------------

type noDriverCall struct {
	deliveryID string
	customerID string
	attempt    int
	reason     string
}

// recordingPublisher captures PublishNoDriverAvailable calls (optionally
// failing) while inheriting the rest of ports.EventPublisher from the shared
// mock.
type recordingPublisher struct {
	*mocks.MockEventPublisher
	mu        sync.Mutex
	noDriver  []noDriverCall
	failPublish bool
}

func newRecordingPublisher() *recordingPublisher {
	return &recordingPublisher{MockEventPublisher: mocks.NewMockEventPublisher()}
}

func (r *recordingPublisher) PublishNoDriverAvailable(ctx context.Context, deliveryID, customerID string, attemptNumber int, reason string) error {
	r.mu.Lock()
	r.noDriver = append(r.noDriver, noDriverCall{
		deliveryID: deliveryID,
		customerID: customerID,
		attempt:    attemptNumber,
		reason:     reason,
	})
	fail := r.failPublish
	r.mu.Unlock()
	if fail {
		return errors.New("simulated event publish failure")
	}
	return nil
}

func (r *recordingPublisher) callsFor(deliveryID string) []noDriverCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []noDriverCall
	for _, c := range r.noDriver {
		if c.deliveryID == deliveryID {
			out = append(out, c)
		}
	}
	return out
}

// failingAssignmentRepo makes only FindRejectedDriverIDsByDelivery fail.
type failingAssignmentRepo struct {
	*mocks.MockAssignmentRepository
	err error
}

func (r *failingAssignmentRepo) FindRejectedDriverIDsByDelivery(ctx context.Context, deliveryID string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.MockAssignmentRepository.FindRejectedDriverIDsByDelivery(ctx, deliveryID)
}

// countingFailingDriverRepo counts fallback lookups and always fails, driving
// the handler's error path (and the consumer retry loop).
type countingFailingDriverRepo struct {
	*mocks.MockDriverRepository
	mu    sync.Mutex
	calls int
}

func (r *countingFailingDriverRepo) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType domain.VehicleType) ([]*domain.Driver, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	return nil, errors.New("simulated geo backend failure")
}

func (r *countingFailingDriverRepo) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// coordFilteredDriverRepo only reports the driver for one exact pickup
// coordinate. All tests share the real delivery.created topic (consumers read
// it from the first offset), so this guarantees messages left over from other
// tests or earlier runs can never reserve the driver before the message under
// test arrives.
type coordFilteredDriverRepo struct {
	*mocks.MockDriverRepository
	wantLat float64
	wantLng float64
}

func (r *coordFilteredDriverRepo) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType domain.VehicleType) ([]*domain.Driver, error) {
	if lat != r.wantLat || lng != r.wantLng {
		return nil, nil
	}
	return r.MockDriverRepository.FindAvailableByLocation(ctx, lat, lng, radiusKm, vehicleType)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func emptyDispatchService() *services.DispatchService {
	return services.NewDispatchService(
		mocks.NewMockDriverRepository(),
		mocks.NewMockAssignmentRepository(),
		mocks.NewMockLocationStore(),
		mocks.NewMockLockManager(),
		mocks.NewMockEventPublisher(),
		domain.NewDispatchPolicy(),
		nil,
	)
}

func deliveryCreatedEnvelope(t *testing.T, eventType string, payload interface{}) []byte {
	t.Helper()
	data, err := events.MarshalEnvelope("evt-"+uniqueName(t), eventType, "", payload)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	return data
}

// publishValues writes each value to topic with a dedicated writer.
func publishValues(t *testing.T, ctx context.Context, topic string, values ...[]byte) {
	t.Helper()
	pub := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
	defer func() { _ = pub.Close() }()
	for _, v := range values {
		if err := pub.PublishMessage(ctx, topic, "", v); err != nil {
			t.Fatalf("publish to %q: %v", topic, err)
		}
	}
}

// runConsumer starts the consumer in the background and returns a stop
// function that cancels the context and asserts Start shut down gracefully.
func runConsumer(t *testing.T, consumer *adapter.DeliveryCreatedConsumer) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- consumer.Start(ctx) }()

	return func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("Start() = %v after cancel, want nil", err)
			}
		case <-time.After(20 * time.Second):
			t.Errorf("Start() did not return after context cancellation")
		}
		if err := consumer.Close(); err != nil {
			t.Logf("consumer Close: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// round-trip tests against the live broker
// ---------------------------------------------------------------------------

// TestDeliveryCreatedConsumer_RoundTrip feeds a realistic message sequence
// (malformed, wrong event type, bad payload, missing id, two valid
// deliveries) through a real consumer group and asserts the valid messages
// reached dispatch while the broken ones were skipped without killing the
// loop.
func TestDeliveryCreatedConsumer_RoundTrip(t *testing.T) {
	probeKafka(t)

	topic := string(events.DeliveryCreated)
	ensureTopics(t, topic)

	runID := uniqueName(t)
	deliveryID := "del-a-" + runID
	customerID := "cust-a-" + runID
	deliveryID2 := "del-b-" + runID
	customerID2 := "cust-b-" + runID

	// One prior assignment for deliveryID => attemptNumber becomes count+1 = 2.
	assignmentRepo := mocks.NewMockAssignmentRepository()
	assignmentRepo.SetAssignment(&domain.Assignment{
		ID:         "asg-seed-" + deliveryID,
		DeliveryID: deliveryID,
		DriverID:   "drv-seed",
		Status:     domain.AssignmentStatusOffered,
	})

	rec := newRecordingPublisher()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	consumer := adapter.NewDeliveryCreatedConsumer(
		[]string{kafkaBroker}, uniqueName(t), emptyDispatchService(), assignmentRepo, rec)
	stop := runConsumer(t, consumer)
	defer stop()

	sequence := [][]byte{
		[]byte("this is not json at all"),
		deliveryCreatedEnvelope(t, "delivery.other.type", map[string]string{"deliveryId": deliveryID}),
		deliveryCreatedEnvelope(t, topic, 123), // payload is not an object
		deliveryCreatedEnvelope(t, topic, map[string]interface{}{"status": "CREATED"}), // no deliveryId
		deliveryCreatedEnvelope(t, topic, map[string]interface{}{
			"deliveryId": deliveryID,
			"customerId": customerID,
			"status":     "CREATED",
			"pickup": map[string]interface{}{
				"latitude":  "30.4429",
				"longitude": 31.1951,
				"location":  map[string]interface{}{"lat": 30.5, "lon": 31.5},
			},
		}),
		deliveryCreatedEnvelope(t, topic, map[string]interface{}{
			"deliveryId": deliveryID2,
			"customerId": customerID2,
			"status":     "CREATED",
			"pickup": map[string]interface{}{
				"location": map[string]interface{}{"lat": 12.5, "lon": 45.5},
			},
		}),
	}
	publishValues(t, ctx, topic, sequence...)

	eventually(t, 45*time.Second, func() bool {
		return len(rec.callsFor(deliveryID)) > 0 && len(rec.callsFor(deliveryID2)) > 0
	}, "no_driver_available events for both valid deliveries")

	calls := rec.callsFor(deliveryID)
	if calls[0].reason != "NO_DRIVERS_NEARBY" {
		t.Errorf("reason = %q, want NO_DRIVERS_NEARBY", calls[0].reason)
	}
	if calls[0].attempt != 2 {
		t.Errorf("attempt = %d, want 2 (1 existing assignment + 1)", calls[0].attempt)
	}
	if calls[0].customerID != customerID {
		t.Errorf("customerID = %q, want %q", calls[0].customerID, customerID)
	}

	calls2 := rec.callsFor(deliveryID2)
	if calls2[0].attempt != 1 {
		t.Errorf("second delivery attempt = %d, want 1", calls2[0].attempt)
	}
	if calls2[0].customerID != customerID2 {
		t.Errorf("second delivery customerID = %q, want %q", calls2[0].customerID, customerID2)
	}
}

// TestDeliveryCreatedConsumer_ReserveDriverRoundTrip covers the happy path:
// an available driver exists, the consumer reserves it and an OFFERED
// assignment is persisted.
func TestDeliveryCreatedConsumer_ReserveDriverRoundTrip(t *testing.T) {
	probeKafka(t)

	topic := string(events.DeliveryCreated)
	ensureTopics(t, topic)

	runID := uniqueName(t)
	deliveryID := "del-reserve-" + runID
	driverID := "drv-reserve-" + runID

	// Unique pickup coordinates so only this run's message (and none of the
	// older messages sitting in the shared topic) can surface the driver.
	key := float64(time.Now().UnixNano()%1_000_000_000)
	wantLat := 30.0 + key/1e6
	wantLng := 40.0 + key/1e6

	driver := &domain.Driver{
		ID:       driverID,
		UserID:   "user-" + runID,
		Status:   domain.DriverStatusAvailable,
		IsActive: true,
		Vehicle:  domain.VehicleInfo{Type: "CAR"},
	}
	driverRepo := &coordFilteredDriverRepo{
		MockDriverRepository: mocks.NewMockDriverRepository(),
		wantLat:              wantLat,
		wantLng:              wantLng,
	}
	driverRepo.SetDriver(driver)
	assignmentRepo := mocks.NewMockAssignmentRepository()
	lockManager := mocks.NewMockLockManager()
	rec := newRecordingPublisher()

	dispatchSvc := services.NewDispatchService(driverRepo, assignmentRepo,
		mocks.NewMockLocationStore(), lockManager, rec, domain.NewDispatchPolicy(), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	consumer := adapter.NewDeliveryCreatedConsumer(
		[]string{kafkaBroker}, uniqueName(t), dispatchSvc, mocks.NewMockAssignmentRepository(), rec)
	stop := runConsumer(t, consumer)
	defer stop()

	publishValues(t, ctx, topic, deliveryCreatedEnvelope(t, topic, map[string]interface{}{
		"deliveryId": deliveryID,
		"customerId": "cust-" + runID,
		"status":     "CREATED",
		"pickup":     map[string]interface{}{"latitude": wantLat, "longitude": wantLng},
	}))

	eventually(t, 45*time.Second, func() bool {
		assignment, err := assignmentRepo.FindByDeliveryID(context.Background(), deliveryID)
		return err == nil && assignment != nil
	}, "assignment to be created for the delivery")

	assignment, err := assignmentRepo.FindByDeliveryID(context.Background(), deliveryID)
	if err != nil {
		t.Fatalf("FindByDeliveryID: %v", err)
	}
	if assignment.DriverID != driverID {
		t.Errorf("assignment driver = %q, want %q", assignment.DriverID, driverID)
	}
	if assignment.Status != domain.AssignmentStatusOffered {
		t.Errorf("assignment status = %q, want %q", assignment.Status, domain.AssignmentStatusOffered)
	}

	d, err := driverRepo.FindByID(context.Background(), driverID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if d == nil || d.Status != domain.DriverStatusBusy {
		t.Errorf("driver status = %v, want BUSY", d)
	}

	wantEvent := "AssignmentOffered:" + deliveryID + "-" + driverID
	found := false
	for _, e := range rec.GetEvents() {
		if e == wantEvent {
			found = true
		}
	}
	if !found {
		t.Errorf("events = %v, want to contain %q", rec.GetEvents(), wantEvent)
	}
}

// TestDeliveryCreatedConsumer_AllDriversBusyRoundTrip locks the only candidate
// so reservation fails: the consumer must fall through to the
// ALL_DRIVERS_BUSY no_driver_available publish with attempt = count+1.
func TestDeliveryCreatedConsumer_AllDriversBusyRoundTrip(t *testing.T) {
	probeKafka(t)

	topic := string(events.DeliveryCreated)
	ensureTopics(t, topic)

	runID := uniqueName(t)
	deliveryID := "del-busy-" + runID
	driverID := "drv-busy-" + runID

	driver := &domain.Driver{
		ID:       driverID,
		Status:   domain.DriverStatusAvailable,
		IsActive: true,
		Vehicle:  domain.VehicleInfo{Type: "CAR"},
	}
	driverRepo := mocks.NewMockDriverRepository()
	driverRepo.SetDriver(driver)
	assignmentRepo := mocks.NewMockAssignmentRepository()
	lockManager := mocks.NewMockLockManager()
	if ok, err := lockManager.Acquire(context.Background(), driverID, time.Minute); !ok || err != nil {
		t.Fatalf("pre-acquire lock: ok=%v err=%v", ok, err)
	}

	// Prior assignment for this delivery => attempt = count+1 = 2.
	assignmentRepo.SetAssignment(&domain.Assignment{
		ID:         "asg-seed-" + deliveryID,
		DeliveryID: deliveryID,
		DriverID:   driverID,
		Status:     domain.AssignmentStatusOffered,
	})

	rec := newRecordingPublisher()
	dispatchSvc := services.NewDispatchService(driverRepo, assignmentRepo,
		mocks.NewMockLocationStore(), lockManager, rec, domain.NewDispatchPolicy(), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	consumer := adapter.NewDeliveryCreatedConsumer(
		[]string{kafkaBroker}, uniqueName(t), dispatchSvc, assignmentRepo, rec)
	stop := runConsumer(t, consumer)
	defer stop()

	publishValues(t, ctx, topic, deliveryCreatedEnvelope(t, topic, map[string]interface{}{
		"deliveryId": deliveryID,
		"customerId": "cust-" + runID,
		"status":     "CREATED",
		"pickup":     map[string]interface{}{"latitude": "30.7", "longitude": "31.7"},
	}))

	eventually(t, 45*time.Second, func() bool {
		return len(rec.callsFor(deliveryID)) > 0
	}, "no_driver_available event after failed reservation")

	call := rec.callsFor(deliveryID)[0]
	if call.reason != "ALL_DRIVERS_BUSY" {
		t.Errorf("reason = %q, want ALL_DRIVERS_BUSY", call.reason)
	}
	if call.attempt != 2 {
		t.Errorf("attempt = %d, want 2", call.attempt)
	}

	// The driver must not have been reserved.
	d, err := driverRepo.FindByID(context.Background(), driverID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if d == nil || d.Status != domain.DriverStatusAvailable {
		t.Errorf("driver status = %v, want AVAILABLE", d)
	}
}

// TestDeliveryCreatedConsumer_DispatchErrorRetries covers the handler error
// path: FindAvailableDrivers keeps failing, the shared consumer retries with
// backoff (2s/4s/8s => at least 4 handler invocations) and the loop still
// shuts down gracefully on cancel.
func TestDeliveryCreatedConsumer_DispatchErrorRetries(t *testing.T) {
	probeKafka(t)

	topic := string(events.DeliveryCreated)
	ensureTopics(t, topic)

	runID := uniqueName(t)
	repo := &countingFailingDriverRepo{MockDriverRepository: mocks.NewMockDriverRepository()}
	dispatchSvc := services.NewDispatchService(repo,
		mocks.NewMockAssignmentRepository(), mocks.NewMockLocationStore(),
		mocks.NewMockLockManager(), mocks.NewMockEventPublisher(),
		domain.NewDispatchPolicy(), nil)

	// nil assignment repo + nil event publisher: exercises the guarded branches.
	consumer := adapter.NewDeliveryCreatedConsumer(
		[]string{kafkaBroker}, uniqueName(t), dispatchSvc, nil, nil)
	stop := runConsumer(t, consumer)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	publishValues(t, ctx, topic, deliveryCreatedEnvelope(t, topic, map[string]interface{}{
		"deliveryId": "del-err-" + runID,
		"customerId": "cust-err-" + runID,
		"status":     "CREATED",
		"pickup":     map[string]interface{}{"latitude": 30.0, "longitude": 31.0},
	}))

	// initial attempt + 3 retries at 2s, 4s and 8s of backoff.
	eventually(t, 60*time.Second, func() bool {
		return repo.callCount() >= 4
	}, "handler to be retried at least 3 times")
}

// TestDeliveryCreatedConsumer_StartCancelGracefully verifies Start returns nil
// once the context is cancelled while no messages are flowing.
func TestDeliveryCreatedConsumer_StartCancelGracefully(t *testing.T) {
	probeKafka(t)

	topic := string(events.DeliveryCreated)
	ensureTopics(t, topic)

	consumer := adapter.NewDeliveryCreatedConsumer(
		[]string{kafkaBroker}, uniqueName(t), emptyDispatchService(), nil, nil)
	stop := runConsumer(t, consumer)
	time.Sleep(500 * time.Millisecond) // let the group join before cancelling
	stop()
}

// ---------------------------------------------------------------------------
// direct handler tests (via the export_test.go hook) for branches that are
// awkward to provoke through a real consumer group
// ---------------------------------------------------------------------------

func TestDeliveryCreatedConsumer_HandleMessageDirect(t *testing.T) {
	ctx := context.Background()
	topic := string(events.DeliveryCreated)

	t.Run("rejected driver lookup failure is logged and ignored", func(t *testing.T) {
		rec := newRecordingPublisher()
		repo := &failingAssignmentRepo{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			err:                      errors.New("mongo unavailable"),
		}
		consumer := adapter.NewDeliveryCreatedConsumer([]string{kafkaBroker}, uniqueName(t),
			emptyDispatchService(), repo, rec)

		deliveryID := "del-rej-" + uniqueName(t)
		msg := kafkago.Message{Value: deliveryCreatedEnvelope(t, topic, map[string]interface{}{
			"deliveryId": deliveryID,
			"customerId": "cust",
			"status":     "CREATED",
		})}
		if err := consumer.HandleMessage(ctx, msg); err != nil {
			t.Fatalf("HandleMessage: %v", err)
		}
		calls := rec.callsFor(deliveryID)
		if len(calls) == 0 {
			t.Fatalf("expected a no_driver_available call, got none")
		}
		if calls[0].reason != "NO_DRIVERS_NEARBY" {
			t.Errorf("reason = %q, want NO_DRIVERS_NEARBY", calls[0].reason)
		}
	})

	t.Run("no_driver publish failure is swallowed", func(t *testing.T) {
		rec := newRecordingPublisher()
		rec.failPublish = true
		consumer := adapter.NewDeliveryCreatedConsumer([]string{kafkaBroker}, uniqueName(t),
			emptyDispatchService(), nil, rec)

		deliveryID := "del-fail-" + uniqueName(t)
		msg := kafkago.Message{Value: deliveryCreatedEnvelope(t, topic, map[string]interface{}{
			"deliveryId": deliveryID,
			"customerId": "cust",
			"status":     "CREATED",
		})}
		if err := consumer.HandleMessage(ctx, msg); err != nil {
			t.Fatalf("HandleMessage = %v, want nil (publish error must be swallowed)", err)
		}
		if len(rec.callsFor(deliveryID)) == 0 {
			t.Errorf("expected the failing publish to have been attempted")
		}
	})

	// Every candidate is offered already (its lock is held), so the reservation
	// loop ends without a reservation and the publisher's failure to announce
	// driver.no_driver_available must be swallowed rather than retried.
	t.Run("all drivers busy publish failure is swallowed", func(t *testing.T) {
		rec := newRecordingPublisher()
		rec.failPublish = true

		const busyDriver = "drv-busy-direct"
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(&domain.Driver{
			ID:       busyDriver,
			Status:   domain.DriverStatusAvailable,
			IsActive: true,
			Vehicle:  domain.VehicleInfo{Type: "CAR"},
		})
		assignmentRepo := mocks.NewMockAssignmentRepository()
		lockManager := mocks.NewMockLockManager()
		if ok, err := lockManager.Acquire(ctx, busyDriver, time.Minute); !ok || err != nil {
			t.Fatalf("pre-acquire lock: ok=%v err=%v", ok, err)
		}
		dispatchSvc := services.NewDispatchService(driverRepo, assignmentRepo,
			mocks.NewMockLocationStore(), lockManager, rec, domain.NewDispatchPolicy(), nil)

		consumer := adapter.NewDeliveryCreatedConsumer([]string{kafkaBroker}, uniqueName(t),
			dispatchSvc, assignmentRepo, rec)

		deliveryID := "del-busy-direct-" + uniqueName(t)
		msg := kafkago.Message{Value: deliveryCreatedEnvelope(t, topic, map[string]interface{}{
			"deliveryId": deliveryID,
			"customerId": "cust",
			"status":     "CREATED",
			"pickup":     map[string]interface{}{"latitude": "30.7", "longitude": "31.7"},
		})}
		if err := consumer.HandleMessage(ctx, msg); err != nil {
			t.Fatalf("HandleMessage = %v, want nil (publish error must be swallowed)", err)
		}

		calls := rec.callsFor(deliveryID)
		if len(calls) == 0 {
			t.Fatal("expected a no_driver_available attempt after every reservation failed")
		}
		if calls[0].reason != "ALL_DRIVERS_BUSY" {
			t.Errorf("reason = %q, want ALL_DRIVERS_BUSY", calls[0].reason)
		}
	})

	t.Run("malformed envelope returns permanent error", func(t *testing.T) {
		consumer := adapter.NewDeliveryCreatedConsumer([]string{kafkaBroker}, uniqueName(t),
			emptyDispatchService(), nil, nil)
		err := consumer.HandleMessage(ctx, kafkago.Message{Value: []byte("{not json")})
		if err == nil {
			t.Fatalf("HandleMessage: got nil error, want error")
		}
		if !errors.Is(err, pkgkafka.ErrPermanent) {
			t.Errorf("error = %v, want a permanent-failure error", err)
		}
	})
}

// parseCoordinate coverage lives in its own table test.
func TestParseCoordinateVariants(t *testing.T) {
	tests := []struct {
		name     string
		val      interface{}
		fallback float64
		want     float64
	}{
		{"nil uses fallback", nil, 1.5, 1.5},
		{"float64", float64(2.5), 0, 2.5},
		{"float32", float32(3.5), 0, 3.5},
		{"int", 4, 0, 4},
		{"int64", int64(5), 0, 5},
		{"numeric string", "6.25", 0, 6.25},
		{"invalid string uses fallback", "not-a-number", 7.5, 7.5},
		{"unsupported type uses fallback", true, 8.5, 8.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.ParseCoordinate(tt.val, tt.fallback)
			if got != tt.want {
				t.Errorf("ParseCoordinate(%v, %v) = %v, want %v", tt.val, tt.fallback, got, tt.want)
			}
		})
	}
}
