package graphql_test

import (
	"testing"

	gql "github.com/graph-gophers/graphql-go"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
)

func TestParseSchema(t *testing.T) {
	_, err := gql.ParseSchema(graphql.DriverSubgraphSDL, &graphql.RootResolver{}, gql.UseStringDescriptions())
	if err != nil {
		t.Fatalf("ParseSchema failed: %v", err)
	}
}
