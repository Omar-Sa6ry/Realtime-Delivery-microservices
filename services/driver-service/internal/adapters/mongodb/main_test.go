package mongodb_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

const (
	mongoURI   = "mongodb://127.0.0.1:27018"
	testDBName = "driver_db_e2e_test"
)

const mongoSkipMsg = "MongoDB is not reachable at " + mongoURI + "; start the e2e stack with " +
	"`docker compose -f docker-compose.e2e.yml -p driver-e2e up -d --wait` from the driver-service directory"

var (
	testMongoClient *mongo.Client
	mongoAvailable  bool
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err == nil {
		if pingErr := client.Ping(ctx, nil); pingErr != nil {
			err = pingErr
		}
	}
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\nskipping all mongodb adapter tests.\n", err)
		mongoAvailable = false
	} else {
		mongoAvailable = true
		testMongoClient = client
	}

	code := m.Run()

	if mongoAvailable {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if derr := testMongoClient.Database(testDBName).Drop(dropCtx); derr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to drop test database %q: %v\n", testDBName, derr)
		}
		_ = testMongoClient.Disconnect(dropCtx)
		dropCancel()
	}
	os.Exit(code)
}

// requireMongo skips the test when the e2e MongoDB is unreachable.
func requireMongo(t *testing.T) *mongo.Client {
	t.Helper()
	if !mongoAvailable {
		t.Skip(mongoSkipMsg)
	}
	return testMongoClient
}

// collectionName builds a unique collection name for a test inside the shared
// test database, so tests cannot interfere with each other.
func collectionName(t *testing.T, base string) string {
	t.Helper()
	safe := strings.NewReplacer("/", "_", `\`, "_", " ", "_", ".", "_").Replace(t.Name())
	return base + "__" + safe
}

// cancelledCtx returns an already-cancelled context for exercising error paths.
func cancelledCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}

// testCollection returns a fresh, uniquely named collection inside the test
// database. It is dropped when the test finishes (the whole database is also
// dropped by TestMain when the suite ends).
func testCollection(t *testing.T, base string) *mongo.Collection {
	t.Helper()
	client := requireMongo(t)
	col := client.Database(testDBName).Collection(collectionName(t, base))
	t.Cleanup(func() { _ = col.Drop(context.Background()) })
	return col
}

// indexNames lists the indexes of a collection as a set of index names.
func indexNames(t *testing.T, col *mongo.Collection) map[string]bool {
	t.Helper()
	ctx := context.Background()
	cursor, err := col.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		t.Fatalf("decode index list: %v", err)
	}
	names := make(map[string]bool, len(docs))
	for _, doc := range docs {
		if name, ok := doc["name"].(string); ok {
			names[name] = true
		}
	}
	return names
}

// asInt64 normalises BSON numeric values for assertions.
func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// idsOf extracts the IDs from a slice of domain pointers for set comparison.
func driverIDs(drivers []*domain.Driver) map[string]bool {
	out := make(map[string]bool, len(drivers))
	for _, d := range drivers {
		out[d.ID] = true
	}
	return out
}
