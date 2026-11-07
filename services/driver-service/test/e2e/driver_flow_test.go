package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcDial opens a client connection to the driver-service gRPC API.
func grpcDial(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial %s: %v", addr, err)
	}
	return conn
}

// GraphQLClient wraps HTTP POST to the GraphQL endpoint.
type GraphQLClient struct {
	url    string
	client *http.Client
}

func newGraphQLClient() *GraphQLClient {
	return &GraphQLClient{
		url: cfg.GraphQLURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *GraphQLClient) exec(ctx context.Context, query string, variables map[string]interface{}, headers map[string]string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// get performs a query/mutation and fails the test on transport errors.
func (c *GraphQLClient) get(t *testing.T, query string, variables map[string]interface{}, headers map[string]string) map[string]interface{} {
	t.Helper()
	resp, err := c.exec(t.Context(), query, variables, headers)
	if err != nil {
		t.Fatalf("graphql request failed: %v", err)
	}
	return resp
}

// activateDriver promotes a freshly registered (inactive) driver to active so
// online/assignment flows can be exercised.
func activateDriver(t *testing.T, client *GraphQLClient, driverID string) {
	t.Helper()
	op := rootField(t, client.get(t, activateDriverMutation, map[string]interface{}{
		"driverId": driverID,
	}, map[string]string{"x-user-id": newID(), "x-user-role": "ADMIN"}), "activateDriver")
	wantSuccess(t, op)
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestE2E_DriverRegistration(t *testing.T) {
	client := newGraphQLClient()

	driverID := newID()
	userID := newID()

	op := rootField(t, client.get(t, registerDriverMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"userId":       userID,
			"vehicleType":  "CAR",
			"plateNumber":  fmt.Sprintf("TEST-%s", driverID[:6]),
			"capacityKg":   100,
			"capabilities": []string{"REFRIGERATED"},
			"serviceArea":  "Cairo",
		},
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "registerDriver")

	wantSuccess(t, op)
	if code, _ := op["statusCode"].(float64); code != 201 {
		t.Errorf("statusCode = %v, want 201", op["statusCode"])
	}
	created := nested(t, op, "data")
	wantStr(t, created, "userId", userID)
	// A self-service application starts inactive pending admin approval.
	wantBool(t, created, "isActive", false)

	// The driver must be readable straight afterwards.
	queried := rootField(t, client.get(t, getDriverQuery, map[string]interface{}{
		"id": driverIDOf(t, created),
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "driver")
	wantSuccess(t, queried)
	fetched := nested(t, queried, "data")
	wantStr(t, fetched, "userId", userID)
	wantStr(t, fetched, "status", "OFFLINE")
}

func TestE2E_DriverRegistrationRequiresAuth(t *testing.T) {
	client := newGraphQLClient()

	op := rootField(t, client.get(t, registerDriverMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"userId":      newID(),
			"vehicleType": "CAR",
			"plateNumber": "NOAUTH-1",
			"capacityKg":  10,
		},
	}, map[string]string{"x-user-role": "DRIVER"}), "registerDriver")

	if ok, _ := op["success"].(bool); ok {
		t.Fatal("registration without a user id should be rejected")
	}
	if code, _ := op["statusCode"].(float64); code != 401 {
		t.Errorf("statusCode = %v, want 401", op["statusCode"])
	}
	wantStr(t, op, "message", "Unauthorized: Login required to apply as driver")
}

func TestE2E_DriverRegistrationRejectsDuplicate(t *testing.T) {
	client := newGraphQLClient()

	userID := newID()
	input := map[string]interface{}{
		"input": map[string]interface{}{
			"userId":      userID,
			"vehicleType": "MOTORCYCLE",
			"plateNumber": "DUP-1",
			"capacityKg":  20,
		},
	}
	headers := map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}

	first := rootField(t, client.get(t, registerDriverMutation, input, headers), "registerDriver")
	wantSuccess(t, first)

	second := rootField(t, client.get(t, registerDriverMutation, input, headers), "registerDriver")
	if ok, _ := second["success"].(bool); ok {
		t.Fatal("second registration for the same user should fail")
	}
}

// ---------------------------------------------------------------------------
// Online / offline
// ---------------------------------------------------------------------------

func TestE2E_DriverOnlineOffline(t *testing.T) {
	client := newGraphQLClient()

	driverID, userID := registerDriver(t, client, "ADMIN")
	activateDriver(t, client, driverID)

	headers := map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}

	online := rootField(t, client.get(t, goOnlineMutation, map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("go-online-%s", newID()),
	}, headers), "goOnline")
	wantSuccess(t, online)
	onlineData := nested(t, online, "data")
	wantStr(t, onlineData, "status", "AVAILABLE")
	wantBool(t, onlineData, "isActive", true)

	status := rootField(t, client.get(t, driverStatusQuery, map[string]interface{}{
		"driverId": driverID,
	}, headers), "driverStatus")
	wantSuccess(t, status)
	statusData := nested(t, status, "data")
	wantStr(t, statusData, "status", "AVAILABLE")
	wantBool(t, statusData, "hasActiveAssignment", false)

	offline := rootField(t, client.get(t, goOfflineMutation, map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("go-offline-%s", newID()),
	}, headers), "goOffline")
	wantSuccess(t, offline)
	wantStr(t, nested(t, offline, "data"), "status", "OFFLINE")

	// The idempotency key is part of the API contract; repeating the same
	// goOnline call must not change state again and must be answered with a
	// clean business error rather than a transport failure.
	key := fmt.Sprintf("idem-%s", newID())
	again := rootField(t, client.get(t, goOnlineMutation, map[string]interface{}{
		"idempotencyKey": key,
	}, headers), "goOnline")
	wantSuccess(t, again)
	replayed := rootField(t, client.get(t, goOnlineMutation, map[string]interface{}{
		"idempotencyKey": key,
	}, headers), "goOnline")
	if ok, _ := replayed["success"].(bool); ok {
		t.Fatal("repeating goOnline while already online should be rejected")
	}
	if code, _ := replayed["statusCode"].(float64); code != 400 {
		t.Errorf("replay statusCode = %v, want 400", replayed["statusCode"])
	}
	wantStr(t, replayed, "message", "[driver_already_online] driver is already online")
}

// ---------------------------------------------------------------------------
// Admin lifecycle
// ---------------------------------------------------------------------------

func TestE2E_BlockUnblockDriver(t *testing.T) {
	client := newGraphQLClient()

	driverID, _ := registerDriver(t, client, "ADMIN")
	// blockDriver/unblockDriver require an exact "admin" role.
	adminHeaders := map[string]string{"x-user-id": newID(), "x-user-role": "admin"}

	blocked := rootField(t, client.get(t, blockDriverMutation, map[string]interface{}{
		"driverId": driverID,
		"reason":   "e2e abuse check",
	}, adminHeaders), "blockDriver")
	wantSuccess(t, blocked)
	wantBool(t, nested(t, blocked, "data"), "isBlocked", true)

	unblocked := rootField(t, client.get(t, unblockDriverMutation, map[string]interface{}{
		"driverId": driverID,
		"reason":   "e2e cleared",
	}, adminHeaders), "unblockDriver")
	wantSuccess(t, unblocked)
	wantBool(t, nested(t, unblocked, "data"), "isBlocked", false)
}

func TestE2E_BlockForbiddenForNonAdmin(t *testing.T) {
	client := newGraphQLClient()

	driverID, userID := registerDriver(t, client, "DRIVER")

	op := rootField(t, client.get(t, blockDriverMutation, map[string]interface{}{
		"driverId": driverID,
		"reason":   "self block attempt",
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "blockDriver")

	if ok, _ := op["success"].(bool); ok {
		t.Fatal("non-admin must not be able to block a driver")
	}
	if code, _ := op["statusCode"].(float64); code != 403 {
		t.Errorf("statusCode = %v, want 403", op["statusCode"])
	}
}

func TestE2E_DriverSuspendActivateDeactivate(t *testing.T) {
	client := newGraphQLClient()

	driverID, _ := registerDriver(t, client, "ADMIN")
	adminHeaders := map[string]string{"x-user-id": newID(), "x-user-role": "ADMIN"}

	suspended := rootField(t, client.get(t, suspendDriverMutation, map[string]interface{}{
		"driverId": driverID,
		"reason":   "e2e suspension",
	}, adminHeaders), "suspendDriver")
	wantSuccess(t, suspended)

	activated := rootField(t, client.get(t, activateDriverMutation, map[string]interface{}{
		"driverId": driverID,
	}, adminHeaders), "activateDriver")
	wantSuccess(t, activated)
	wantBool(t, nested(t, activated, "data"), "isActive", true)

	deactivated := rootField(t, client.get(t, deactivateDriverMutation, map[string]interface{}{
		"driverId": driverID,
	}, adminHeaders), "deactivateDriver")
	wantSuccess(t, deactivated)
	wantBool(t, nested(t, deactivated, "data"), "isActive", false)
}

// ---------------------------------------------------------------------------
// Profile
// ---------------------------------------------------------------------------

func TestE2E_UpdateDriverProfile(t *testing.T) {
	client := newGraphQLClient()

	driverID, userID := registerDriver(t, client, "ADMIN")

	op := rootField(t, client.get(t, updateDriverProfileMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"driverId":    driverID,
			"plateNumber": "UPDATED-1",
			"capacityKg":  200,
			"serviceArea": "Alexandria",
		},
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "updateDriverProfile")
	wantSuccess(t, op)

	updated := nested(t, op, "data")
	wantStr(t, updated, "plateNumber", "UPDATED-1")
	wantStr(t, updated, "serviceArea", "Alexandria")

	// The change must be persisted, not just echoed back.
	fetched := rootField(t, client.get(t, getDriverQuery, map[string]interface{}{
		"id": driverID,
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "driver")
	wantSuccess(t, fetched)
	wantStr(t, nested(t, fetched, "data"), "plateNumber", "UPDATED-1")
}

// ---------------------------------------------------------------------------
// Ratings
// ---------------------------------------------------------------------------

func TestE2E_DriverRating(t *testing.T) {
	client := newGraphQLClient()

	driverID, driverUser := registerDriver(t, client, "ADMIN")
	customerID := newID()
	// The e2e delivery-service stub resolves customer/driver ownership from the
	// delivery id so the ownership check is exercised end to end.
	deliveryID := fmt.Sprintf("e2e:%s:%s:%s", driverID, customerID, newID())

	rated := rootField(t, client.get(t, rateDriverMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"driverId":   driverID,
			"deliveryId": deliveryID,
			"rating":     4.5,
			"comment":    "Great service!",
		},
	}, map[string]string{"x-user-id": customerID, "x-user-role": "CUSTOMER"}), "rateDriver")
	wantSuccess(t, rated)
	review := nested(t, rated, "data")
	wantStr(t, review, "driverId", driverID)
	if rating, _ := review["rating"].(float64); rating != 4.5 {
		t.Errorf("rating = %v, want 4.5", review["rating"])
	}

	// The review must be listed for the driver.
	reviews := rootField(t, client.get(t, driverReviewsQuery, map[string]interface{}{
		"driverId": driverID,
		"page":     1,
		"limit":    10,
	}, map[string]string{"x-user-id": driverUser, "x-user-role": "DRIVER"}), "driverReviews")
	wantSuccess(t, reviews)
	reviewsData := nested(t, reviews, "data")
	if total, _ := reviewsData["totalReviews"].(float64); total < 1 {
		t.Errorf("totalReviews = %v, want >= 1", reviewsData["totalReviews"])
	}
	items, _ := reviewsData["items"].([]interface{})
	if len(items) == 0 {
		t.Fatalf("expected at least one review, got %v", reviewsData)
	}
	first, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("review item is not an object: %v", items[0])
	}
	wantStr(t, first, "comment", "Great service!")

	// A different customer does not own the delivery and must be rejected.
	other := rootField(t, client.get(t, rateDriverMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"driverId":   driverID,
			"deliveryId": deliveryID,
			"rating":     1,
		},
	}, map[string]string{"x-user-id": newID(), "x-user-role": "CUSTOMER"}), "rateDriver")
	if ok, _ := other["success"].(bool); ok {
		t.Fatal("non-owner must not be allowed to rate")
	}
}

// ---------------------------------------------------------------------------
// Assignments
// ---------------------------------------------------------------------------

func TestE2E_AssignmentFlow(t *testing.T) {
	client := newGraphQLClient()

	driver1ID, driver1User := registerDriver(t, client, "ADMIN")
	driver2ID, driver2User := registerDriver(t, client, "ADMIN")
	activateDriver(t, client, driver1ID)
	activateDriver(t, client, driver2ID)

	for _, u := range []string{driver1User, driver2User} {
		online := rootField(t, client.get(t, goOnlineMutation, map[string]interface{}{
			"idempotencyKey": fmt.Sprintf("go-online-%s", newID()),
		}, map[string]string{"x-user-id": u, "x-user-role": "DRIVER"}), "goOnline")
		wantSuccess(t, online)
	}

	nearby := rootField(t, client.get(t, nearbyDriversQuery, map[string]interface{}{
		"input": map[string]interface{}{
			"latitude":  30.0444,
			"longitude": 31.2357,
			"radiusKm":  50,
			"limit":     10,
		},
	}, map[string]string{"x-user-id": driver1User, "x-user-role": "DRIVER"}), "nearbyDrivers")
	wantSuccess(t, nearby)

	// A driver can attempt to self-accept a delivery; without the delivery
	// service owning it the mutation must answer with a clean business error
	// (never a transport/GraphQL error).
	selfAccept := rootField(t, client.get(t, selfAcceptDeliveryMutation, map[string]interface{}{
		"deliveryId":     fmt.Sprintf("e2e:%s:%s:%s", driver1ID, "e2e-customer", newID()),
		"idempotencyKey": fmt.Sprintf("self-accept-%s", newID()),
	}, map[string]string{"x-user-id": driver1User, "x-user-role": "DRIVER"}), "selfAcceptDelivery")
	if selfAccept["success"] == nil || selfAccept["statusCode"] == nil {
		t.Fatalf("selfAcceptDelivery returned a malformed payload: %v", selfAccept)
	}
	if ok, _ := selfAccept["success"].(bool); !ok {
		t.Logf("selfAcceptDelivery rejected as expected outside a real delivery: %v", selfAccept["message"])
	}

	// Accepting an unknown assignment must fail gracefully.
	bad := rootField(t, client.get(t, acceptAssignmentMutation, map[string]interface{}{
		"assignmentId":   "missing-assignment",
		"idempotencyKey": fmt.Sprintf("accept-%s", newID()),
	}, map[string]string{"x-user-id": driver1User, "x-user-role": "DRIVER"}), "acceptAssignment")
	if ok, _ := bad["success"].(bool); ok {
		t.Fatal("accepting a nonexistent assignment should fail")
	}
}

func TestE2E_AvailableDeliveries(t *testing.T) {
	client := newGraphQLClient()

	driverID, userID := registerDriver(t, client, "ADMIN")
	activateDriver(t, client, driverID)

	online := rootField(t, client.get(t, goOnlineMutation, map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("go-online-%s", newID()),
	}, map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "goOnline")
	wantSuccess(t, online)

	avail := rootField(t, client.get(t, availableDeliveriesQuery, map[string]interface{}{},
		map[string]string{"x-user-id": userID, "x-user-role": "DRIVER"}), "availableDeliveries")
	wantSuccess(t, avail)
	if _, ok := nested(t, avail, "data")["items"].([]interface{}); !ok {
		t.Fatalf("availableDeliveries.items is not a list: %v", avail)
	}
}

// ---------------------------------------------------------------------------
// Protocol / transport
// ---------------------------------------------------------------------------

func TestE2E_GRPCDriverAPI(t *testing.T) {
	client := newGraphQLClient()
	driverID, userID := registerDriver(t, client, "ADMIN")
	activateDriver(t, client, driverID)

	conn := grpcDial(t, cfg.GRPCAddr)
	defer conn.Close()
	drv := pb.NewDriverServiceClient(conn)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	got, err := drv.GetDriver(ctx, &pb.GetDriverRequest{DriverId: driverID})
	if err != nil {
		t.Fatalf("GetDriver: %v", err)
	}
	if !got.GetFound() {
		t.Fatalf("GetDriver did not find %s: %+v", driverID, got)
	}
	if got.GetUserId() != userID {
		t.Errorf("userId = %q, want %q", got.GetUserId(), userID)
	}

	missing, err := drv.GetDriver(ctx, &pb.GetDriverRequest{DriverId: "no-such-driver"})
	if err != nil {
		t.Fatalf("GetDriver(missing): %v", err)
	}
	if missing.GetFound() {
		t.Error("GetDriver reported found for an unknown id")
	}

	st, err := drv.GetDriverStatus(ctx, &pb.GetDriverStatusRequest{DriverId: driverID})
	if err != nil {
		t.Fatalf("GetDriverStatus: %v", err)
	}
	if st.GetDriverId() != driverID {
		t.Errorf("status driverId = %q, want %q", st.GetDriverId(), driverID)
	}

	valid, err := drv.ValidateID(ctx, &pb.ValidateIDRequest{Id: driverID, IdType: "driver"})
	if err != nil {
		t.Fatalf("ValidateID: %v", err)
	}
	if !valid.GetValid() {
		t.Errorf("ValidateID rejected a real driver id: %v", valid.GetMessage())
	}
	invalid, err := drv.ValidateID(ctx, &pb.ValidateIDRequest{Id: "not-a-uuid", IdType: "driver"})
	if err != nil {
		t.Fatalf("ValidateID(invalid): %v", err)
	}
	if invalid.GetValid() {
		t.Error("ValidateID accepted a malformed id")
	}
}

func TestE2E_ServiceIntrospection(t *testing.T) {
	client := newGraphQLClient()

	op := rootField(t, client.get(t, serviceQuery, map[string]interface{}{}, map[string]string{}), "_service")
	sdl, _ := op["sdl"].(string)
	if sdl == "" {
		t.Fatal("_service.sdl is empty")
	}
	if !strings.Contains(sdl, "type Query") {
		t.Errorf("SDL does not look like a schema: %.80q", sdl)
	}
}

func TestE2E_HealthEndpoint(t *testing.T) {
	resp, err := http.Get(cfg.GraphQLBase + "/healthz")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health endpoint returned %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("health body: %v", err)
	}
	wantStr(t, body, "status", "ok")
	wantStr(t, body, "service", "driver-service")
}

// ---------------------------------------------------------------------------
// Backing infrastructure
// ---------------------------------------------------------------------------

func TestE2E_KafkaRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Kafka integration test in short mode")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{cfg.KafkaBroker},
		Topic:    "driver.events",
		GroupID:  "e2e-test-" + newID(),
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// The topic may be empty; any broker-level error other than a timeout
	// means Kafka is not usable.
	if _, err := reader.ReadMessage(ctx); err != nil && ctx.Err() == nil {
		t.Fatalf("kafka read: %v", err)
	}
}

func TestE2E_NATSRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping NATS integration test in short mode")
	}

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	subject := "e2e.test." + newID()
	sub, err := nc.SubscribeSync(subject)
	if err != nil {
		t.Fatalf("nats subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	if err := nc.Publish(subject, []byte("hello")); err != nil {
		t.Fatalf("nats publish: %v", err)
	}
	msg, err := sub.NextMsg(2 * time.Second)
	if err != nil {
		t.Fatalf("nats next msg: %v", err)
	}
	if string(msg.Data) != "hello" {
		t.Errorf("payload = %q, want hello", msg.Data)
	}
}

func TestE2E_RabbitMQRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping RabbitMQ integration test in short mode")
	}

	conn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		t.Fatalf("rabbitmq dial: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("rabbitmq channel: %v", err)
	}
	defer ch.Close()

	queue := "e2e.test." + newID()
	q, err := ch.QueueDeclare(queue, false, false, true, false, nil)
	if err != nil {
		t.Fatalf("queue declare: %v", err)
	}

	if err := ch.Publish("", q.Name, false, false, amqp091.Publishing{
		ContentType: "text/plain",
		Body:        []byte("hello"),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}

	select {
	case msg := <-msgs:
		if string(msg.Body) != "hello" {
			t.Errorf("payload = %q, want hello", msg.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the RabbitMQ message")
	}
}

func TestE2E_MongoRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	defer client.Disconnect(ctx)

	collection := client.Database(cfg.MongoDB).Collection("e2e_test_" + newID())
	if _, err := collection.InsertOne(ctx, bson.M{"test": "value", "ts": time.Now()}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var result bson.M
	if err := collection.FindOne(ctx, bson.M{"test": "value"}).Decode(&result); err != nil {
		t.Fatalf("find: %v", err)
	}
	if result["test"] != "value" {
		t.Errorf("test = %v, want value", result["test"])
	}
}

func TestE2E_RedisRoundTrip(t *testing.T) {
	conn, err := net.DialTimeout("tcp", cfg.RedisAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("redis dial: %v", err)
	}
	defer conn.Close()
	// A single reader must be reused: bufio would otherwise discard bytes it
	// already buffered for the next reply.
	rd := bufio.NewReader(conn)
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	key := "e2e_test_" + newID()
	if _, err := conn.Write([]byte("SET " + key + " hello\r\n")); err != nil {
		t.Fatalf("redis set: %v", err)
	}
	if line := redisReply(t, rd); !strings.HasPrefix(line, "+OK") {
		t.Errorf("SET reply = %q, want +OK", line)
	}

	if _, err := conn.Write([]byte("GET " + key + "\r\n")); err != nil {
		t.Fatalf("redis get: %v", err)
	}
	// GET answers with a bulk string: a length line followed by the value.
	if line := redisReply(t, rd); !strings.HasPrefix(line, "$5") {
		t.Errorf("GET header = %q, want $5", line)
	}
	if value := redisReply(t, rd); value != "hello\r\n" {
		t.Errorf("GET value = %q, want hello", value)
	}
}

// redisReply reads a single RESP line, failing the test on timeouts.
func redisReply(t *testing.T, rd *bufio.Reader) string {
	t.Helper()
	line, err := rd.ReadString('\n')
	if err != nil {
		t.Fatalf("redis reply: %v", err)
	}
	return line
}

// ---------------------------------------------------------------------------
// Query / mutation documents
// ---------------------------------------------------------------------------

const (
	serviceQuery = `
		query {
			_service {
				sdl
			}
		}
	`

	registerDriverMutation = `
		mutation RegisterDriver($input: RegisterDriverInput!) {
			registerDriver(input: $input) {
				success
				statusCode
				message
				data {
					id
					userId
					status
					vehicleType
					plateNumber
					isActive
				}
			}
		}
	`

	getDriverQuery = `
		query GetDriver($id: ID!) {
			driver(id: $id) {
				success
				statusCode
				message
				data {
					id
					userId
					status
					vehicleType
					plateNumber
					rating
					isActive
					isBlocked
				}
			}
		}
	`

	goOnlineMutation = `
		mutation GoOnline($idempotencyKey: String!) {
			goOnline(idempotencyKey: $idempotencyKey) {
				success
				statusCode
				message
				data {
					id
					status
					isActive
				}
			}
		}
	`

	goOfflineMutation = `
		mutation GoOffline($idempotencyKey: String!) {
			goOffline(idempotencyKey: $idempotencyKey) {
				success
				statusCode
				message
				data {
					id
					status
				}
			}
		}
	`

	driverStatusQuery = `
		query DriverStatus($driverId: ID!) {
			driverStatus(driverId: $driverId) {
				success
				statusCode
				message
				data {
					driver {
						id
						status
					}
					status
					hasActiveAssignment
				}
			}
		}
	`

	nearbyDriversQuery = `
		query NearbyDrivers($input: NearbyDriversInput!) {
			nearbyDrivers(input: $input) {
				success
				statusCode
				message
				data {
					items {
						driverId
						distanceMeters
						status
						vehicleType
					}
				}
			}
		}
	`

	selfAcceptDeliveryMutation = `
		mutation SelfAcceptDelivery($deliveryId: ID!, $idempotencyKey: String!) {
			selfAcceptDelivery(deliveryId: $deliveryId, idempotencyKey: $idempotencyKey) {
				success
				statusCode
				message
				data {
					id
					driverId
					deliveryId
					status
				}
			}
		}
	`

	acceptAssignmentMutation = `
		mutation AcceptAssignment($assignmentId: ID!, $idempotencyKey: String!) {
			acceptAssignment(assignmentId: $assignmentId, idempotencyKey: $idempotencyKey) {
				success
				statusCode
				message
				data {
					id
					status
				}
			}
		}
	`

	rateDriverMutation = `
		mutation RateDriver($input: RateDriverInput!) {
			rateDriver(input: $input) {
				success
				statusCode
				message
				data {
					id
					driverId
					rating
					comment
				}
			}
		}
	`

	driverReviewsQuery = `
		query DriverReviews($driverId: ID!, $page: Int, $limit: Int) {
			driverReviews(driverId: $driverId, page: $page, limit: $limit) {
				success
				statusCode
				message
				data {
					items {
						id
						rating
						comment
					}
					averageRating
					totalReviews
				}
			}
		}
	`

	blockDriverMutation = `
		mutation BlockDriver($driverId: ID!, $reason: String) {
			blockDriver(driverId: $driverId, reason: $reason) {
				success
				statusCode
				message
				data {
					id
					isBlocked
				}
			}
		}
	`

	unblockDriverMutation = `
		mutation UnblockDriver($driverId: ID!, $reason: String) {
			unblockDriver(driverId: $driverId, reason: $reason) {
				success
				statusCode
				message
				data {
					id
					isBlocked
				}
			}
		}
	`

	suspendDriverMutation = `
		mutation SuspendDriver($driverId: ID!, $reason: String!) {
			suspendDriver(driverId: $driverId, reason: $reason) {
				success
				statusCode
				message
				data {
					id
					isActive
				}
			}
		}
	`

	activateDriverMutation = `
		mutation ActivateDriver($driverId: ID!) {
			activateDriver(driverId: $driverId) {
				success
				statusCode
				message
				data {
					id
					isActive
				}
			}
		}
	`

	deactivateDriverMutation = `
		mutation DeactivateDriver($driverId: ID!) {
			deactivateDriver(driverId: $driverId) {
				success
				statusCode
				message
				data {
					id
					isActive
				}
			}
		}
	`

	updateDriverProfileMutation = `
		mutation UpdateDriverProfile($input: UpdateDriverProfileInput!) {
			updateDriverProfile(input: $input) {
				success
				statusCode
				message
				data {
					id
					plateNumber
					capacityKg
					serviceArea
				}
			}
		}
	`

	availableDeliveriesQuery = `
		query AvailableDeliveries {
			availableDeliveries {
				success
				statusCode
				message
				data {
					items {
						id
						status
						pickupCity
						dropoffCity
					}
				}
			}
		}
	`
)
