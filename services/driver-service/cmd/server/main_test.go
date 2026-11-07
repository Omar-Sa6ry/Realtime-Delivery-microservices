package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	pkgrmq "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	rmqadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/rabbitmq"
)

// e2eRabbitURL is the broker of docker-compose.e2e.yml (offset from the
// production port 5672 so a local RabbitMQ is never touched).
const e2eRabbitURL = "amqp://guest:guest@127.0.0.1:5673/"

// dispatchExchange is the exchange the entrypoint declares on start-up.
const dispatchExchange = "delivery.dispatch.direct"

// probe skips the test when the live infrastructure the service needs (MongoDB
// and Kafka) is not reachable, so a stopped Docker daemon never fails the build.
func probeInfra(t *testing.T) {
	t.Helper()
	for _, addr := range []string{"127.0.0.1:27018", "127.0.0.1:9094", "127.0.0.1:6380"} {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			t.Skipf("infrastructure at %s is not reachable (%v); "+
				"run `docker compose -f docker-compose.e2e.yml -p driver-e2e up -d --wait`", addr, err)
		}
		_ = conn.Close()
	}
}

// freePort asks the kernel for an unused port so the test never collides with
// a service that is already running on the default ports.
func freePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for a free port: %v", err)
	}
	defer lis.Close()
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	return port
}

// setServiceEnv points the service at the e2e stack on free ports. Every
// scenario starts from this baseline and overrides only what it exercises.
func setServiceEnv(t *testing.T) (graphqlPort, grpcPort, metricsPort string) {
	t.Helper()
	graphqlPort, grpcPort, metricsPort = freePort(t), freePort(t), freePort(t)

	t.Setenv("PORT_DRIVER_GRAPHQL", graphqlPort)
	t.Setenv("PORT_DRIVER_GRPC", grpcPort)
	t.Setenv("PORT_DRIVER_METRICS", metricsPort)
	t.Setenv("MONGODB_URI", "mongodb://127.0.0.1:27018")
	t.Setenv("MONGODB_DATABASE", "driver_db_e2e_test")
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("KAFKA_BROKERS", "127.0.0.1:9094")
	t.Setenv("KAFKA_GROUP_ID", fmt.Sprintf("driver-service-cmdtest-%d", time.Now().UnixNano()))
	t.Setenv("KAFKA_CLIENT_ID", "driver-service-cmdtest")
	t.Setenv("NATS_URL", "nats://127.0.0.1:4223")
	t.Setenv("USER_SERVICE_URL", "127.0.0.1:50051")
	t.Setenv("DELIVERY_SERVICE_URL", "127.0.0.1:50051")
	t.Setenv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5673/")
	// No collector is listening on 4318; the exporter is still constructed
	// successfully, which is what the tracer/shutdown branches need.
	t.Setenv("OTEL_ENDPOINT", "127.0.0.1:4318")
	return graphqlPort, grpcPort, metricsPort
}

// ---------------------------------------------------------------------------
// In-process: the happy path
// ---------------------------------------------------------------------------

// TestRun_StartupAndGracefulShutdown exercises the real service entrypoint in
// process: it must wire everything together, answer health and metrics
// endpoints, consume a dispatch request off RabbitMQ, and return exit code 0
// once its context is cancelled.
func TestRun_StartupAndGracefulShutdown(t *testing.T) {
	probeInfra(t)

	graphqlPort, _, metricsPort := setServiceEnv(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan int, 1)
	go func() { exit <- run(ctx) }()

	// The GraphQL server must come up and report healthy.
	healthURL := fmt.Sprintf("http://127.0.0.1:%s/healthz", graphqlPort)
	waitHealthy(t, healthURL)

	// The metrics endpoint must serve Prometheus metrics on its own port.
	metricsURL := fmt.Sprintf("http://127.0.0.1:%s/metrics", metricsPort)
	resp, err := http.Get(metricsURL)
	if err != nil {
		t.Fatalf("metrics endpoint: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics endpoint status = %d, want 200", resp.StatusCode)
	}
	if len(body) == 0 {
		t.Error("metrics endpoint returned an empty body")
	}

	// A dispatch request must reach the RabbitMQ consumer the entrypoint wired.
	publishDispatchRequest(t)

	// Cancelling the context must shut the service down with exit code 0.
	cancel()
	select {
	case code := <-exit:
		if code != 0 {
			t.Errorf("run() exit code = %d, want 0", code)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("run() did not return after its context was cancelled")
	}
}

// waitHealthy polls a URL until it answers 200 or the deadline passes.
func waitHealthy(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && len(body) > 0 {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("service did not become healthy at %s: err=%v", url, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// publishDispatchRequest sends one dispatch.request through the live broker
// and waits until it leaves the queue, which only happens once the service's
// consumer took and acknowledged it.
//
// Run this package on its own: while another driver-service instance (the
// e2e one on port 4008) is consuming the same queue, it can take the request
// instead of the process under test.
func publishDispatchRequest(t *testing.T) {
	t.Helper()
	conn, err := amqp.Dial("amqp://guest:guest@127.0.0.1:5673/")
	if err != nil {
		t.Fatalf("dial RabbitMQ: %v", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		t.Fatalf("confirm mode: %v", err)
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))

	const (
		exchange   = "delivery.dispatch.direct"
		routingKey = "dispatch.request"
		queue      = "dispatch.requests.queue"
	)
	if err := ch.Publish(exchange, routingKey, true, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(`{"eventType":"dispatch.request","eventId":"evt-cmd-test"}`),
	}); err != nil {
		t.Fatalf("publish dispatch request: %v", err)
	}
	// The confirm proves the broker queued the request; without it the depth
	// poll below could observe the queue before the publish was processed and
	// report an empty queue that never carried a message.
	select {
	case c := <-confirms:
		if !c.Ack {
			t.Fatal("broker rejected the dispatch request")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("no confirm for the dispatch request")
	}
	// basic.ack is preceded by basic.return for unroutable mandatory
	// publishes, so an empty return queue proves the request reached the queue.
	select {
	case ret := <-returns:
		t.Fatalf("dispatch request was not routable: %+v", ret)
	default:
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		q, perr := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
		if perr != nil {
			t.Fatalf("passive queue declare: %v", perr)
		}
		depth := q.Messages
		if depth == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dispatch request still queued after 15s (depth=%d)", depth)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// The request left the queue; give the consumer's handler a moment to run
	// before the test tears the service down.
	time.Sleep(300 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// Child scenarios: each run() can only start once per process because the
// Prometheus collectors are registered on the default registry, so every
// scenario runs in a re-executed copy of this test binary.
// ---------------------------------------------------------------------------

var scenarios = map[string]func(t *testing.T) int{
	"busy-grpc-port":    busyGRPCPortScenario,
	"invalid-mongo-uri": invalidMongoURIScenario,
	"unreachable-infra": unreachableInfraScenario,
	"busy-http-ports":   busyHTTPPortsScenario,
	"topology-conflict": topologyConflictScenario,
	"main-exit-code":    mainExitCodeScenario,
}

// TestRun_ScenarioChild is the entry point of the re-executed test binary. In
// a normal run it does nothing: the parent tests spawn it with the scenario
// name in DRIVER_CMD_SCENARIO and read the reported exit code.
func TestRun_ScenarioChild(t *testing.T) {
	name := os.Getenv("DRIVER_CMD_SCENARIO")
	if name == "" {
		t.Skip("scenario children are spawned by the parent tests")
	}
	scenario, ok := scenarios[name]
	if !ok {
		t.Fatalf("unknown scenario %q", name)
	}
	fmt.Printf("CHILD_EXIT=%d\n", scenario(t))
	// The scenario's coverage only reaches GOCOVERDIR when the process ends
	// through os.Exit: letting the testing package return normally discards
	// the counters of this child process.
	os.Exit(0)
}

// spawnScenario re-executes this test binary for the named scenario and
// returns its stdout plus the process exit code.
func spawnScenario(t *testing.T, name string) (int, string) {
	t.Helper()
	probeInfra(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestRun_ScenarioChild$")
	cmd.Env = append(os.Environ(), "DRIVER_CMD_SCENARIO="+name)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("spawn scenario %q: %v\n%s", name, err, out)
		}
		code = exitErr.ExitCode()
	}
	return code, string(out)
}

// wantChildExit fails unless the child reported the expected scenario code.
func wantChildExit(t *testing.T, name string, want int) {
	t.Helper()
	code, out := spawnScenario(t, name)
	if code != 0 {
		t.Fatalf("scenario %q exited with %d, want 0 (it must report its code on stdout)\n%s", name, code, out)
	}
	if !strings.Contains(out, fmt.Sprintf("CHILD_EXIT=%d", want)) {
		t.Errorf("scenario %q did not report CHILD_EXIT=%d\n%s", name, want, out)
	}
}

// TestRun_RejectsBusyGRPCPort covers the bind-failure branch of the entrypoint:
// run() must return exit code 1 instead of panicking or hanging.
func TestRun_RejectsBusyGRPCPort(t *testing.T) {
	wantChildExit(t, "busy-grpc-port", 1)
}

// TestRun_ReportsInvalidMongoURI covers the mongo.Connect failure branch, which
// returns before any server is started.
func TestRun_ReportsInvalidMongoURI(t *testing.T) {
	wantChildExit(t, "invalid-mongo-uri", 1)
}

// TestRun_SurvivesUnreachableInfrastructure covers the non-fatal warning
// branches: a MongoDB that does not answer, a Kafka broker that cannot be
// dialled (topic creation and the delivery consumer both fail and retry) must
// be logged rather than crash the service.
func TestRun_SurvivesUnreachableInfrastructure(t *testing.T) {
	wantChildExit(t, "unreachable-infra", 0)
}

// TestRun_LogsBusyHTTPPorts covers the ListenAndServe error branches of the
// GraphQL and metrics servers: their ports are held by the test, so both
// goroutines must log the bind failure while run() keeps serving.
func TestRun_LogsBusyHTTPPorts(t *testing.T) {
	wantChildExit(t, "busy-http-ports", 0)
}

// TestRun_ContinuesWhenTopologyConflicts covers the non-fatal RabbitMQ
// topology warning: with an exchange of the wrong kind in the way,
// EnsureDriverTopology fails and run() must log it and keep serving.
func TestRun_ContinuesWhenTopologyConflicts(t *testing.T) {
	wantChildExit(t, "topology-conflict", 0)
}

// TestMain_UsesRunExitCode covers main() itself: it has to turn run()'s exit
// code into the process exit status, so the child must die with 1.
func TestMain_UsesRunExitCode(t *testing.T) {
	code, out := spawnScenario(t, "main-exit-code")
	if code != 1 {
		t.Errorf("main() exit status = %d, want 1\n%s", code, out)
	}
}

// busyGRPCPortScenario occupies the wildcard gRPC port, which run() must fail
// to bind (it listens on ":"+port, so a 127.0.0.1 listener is not enough).
func busyGRPCPortScenario(t *testing.T) int {
	t.Helper()
	port := freePort(t)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("listen on :%s: %v", port, err)
	}
	defer lis.Close()

	setServiceEnv(t)
	t.Setenv("PORT_DRIVER_GRPC", port)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return run(ctx)
}

// invalidMongoURIScenario hands run() a connection string that cannot be
// parsed, which must abort the start-up with exit code 1.
func invalidMongoURIScenario(t *testing.T) int {
	t.Helper()
	setServiceEnv(t)
	t.Setenv("MONGODB_URI", "://not-a-mongodb-uri")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return run(ctx)
}

// unreachableInfraScenario points MongoDB and Kafka at dead ports: both are
// non-fatal in run(), which must keep serving until the context is cancelled.
func unreachableInfraScenario(t *testing.T) int {
	t.Helper()
	setServiceEnv(t)
	t.Setenv("MONGODB_URI", "mongodb://127.0.0.1:1")
	t.Setenv("KAFKA_BROKERS", "127.0.0.1:1")
	// RabbitMQ retries its dial for minutes, so leave it out of this scenario.
	_ = os.Unsetenv("RABBITMQ_URL")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(6 * time.Second)
		cancel()
	}()
	return run(ctx)
}

// busyHTTPPortsScenario holds the GraphQL and metrics ports so both
// ListenAndServe goroutines hit their error branch.
func busyHTTPPortsScenario(t *testing.T) int {
	t.Helper()
	graphqlPort, _, metricsPort := setServiceEnv(t)

	graphqlLis, err := net.Listen("tcp", ":"+graphqlPort)
	if err != nil {
		t.Fatalf("hold graphql port: %v", err)
	}
	defer graphqlLis.Close()
	metricsLis, err := net.Listen("tcp", ":"+metricsPort)
	if err != nil {
		t.Fatalf("hold metrics port: %v", err)
	}
	defer metricsLis.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(3 * time.Second)
		cancel()
	}()
	return run(ctx)
}

// mainExitCodeScenario calls main(), which must end the process with run()'s
// exit code. The gRPC port is held so run() answers 1; os.Exit therefore never
// returns and nothing is printed afterwards.
func mainExitCodeScenario(t *testing.T) int {
	t.Helper()
	port := freePort(t)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("listen on :%s: %v", port, err)
	}
	defer lis.Close()

	setServiceEnv(t)
	t.Setenv("PORT_DRIVER_GRPC", port)

	main()
	return -1 // unreachable: main() exits the process
}

// topologyConflictScenario makes EnsureDriverTopology fail by putting an
// exchange of the wrong kind in its way. run() must treat that as a warning
// and serve until its context is cancelled.
func topologyConflictScenario(t *testing.T) int {
	t.Helper()
	setServiceEnv(t)

	// Restoring the exchange is what keeps the rest of the suite (and the
	// e2e services) able to publish dispatch requests again, so it has to run
	// no matter how the scenario ends.
	defer conflictDispatchExchange(t)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(4 * time.Second)
		cancel()
	}()
	return run(ctx)
}

// conflictDispatchExchange replaces the dispatch exchange with a fanout
// exchange of the same name (an exchange cannot change kind, so the original
// one is deleted first, which also drops its bindings) and returns a function
// that puts the direct exchange, the queues and every binding back.
func conflictDispatchExchange(t *testing.T) func() {
	t.Helper()

	conn, err := amqp.Dial(e2eRabbitURL)
	if err != nil {
		t.Fatalf("dial RabbitMQ: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		t.Fatalf("channel: %v", err)
	}
	if err := ch.ExchangeDelete(dispatchExchange, false, false); err != nil {
		t.Fatalf("delete dispatch exchange: %v", err)
	}
	if err := ch.ExchangeDeclare(dispatchExchange, "fanout", true, false, false, false, nil); err != nil {
		t.Fatalf("declare conflicting exchange: %v", err)
	}
	_ = ch.Close()
	_ = conn.Close()

	return func() {
		restoreDispatchTopology(t)
	}
}

// restoreDispatchTopology removes the conflicting exchange and re-runs the
// service's own topology declaration, which recreates the direct exchange,
// the queues and their bindings exactly as production expects them.
func restoreDispatchTopology(t *testing.T) {
	t.Helper()

	conn, err := amqp.Dial(e2eRabbitURL)
	if err != nil {
		t.Errorf("reconnect to restore the topology: %v", err)
		return
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		t.Errorf("channel: %v", err)
		return
	}
	if err := ch.ExchangeDelete(dispatchExchange, false, false); err != nil {
		t.Errorf("delete conflicting exchange: %v", err)
	}
	_ = ch.Close()
	_ = conn.Close()

	shared, err := pkgrmq.ConnectURL(e2eRabbitURL, "topology-restore")
	if err != nil {
		t.Errorf("connect for topology restore: %v", err)
		return
	}
	defer shared.Close()

	for i := 0; i < 2; i++ {
		// The first pass rebuilds everything; the second one only succeeds if
		// the exchange really is a direct exchange again, because a kind
		// mismatch would make the declaration fail.
		if err := rmqadapter.EnsureDriverTopology(shared); err != nil {
			t.Errorf("restore dispatch topology: %v", err)
			return
		}
	}
}
