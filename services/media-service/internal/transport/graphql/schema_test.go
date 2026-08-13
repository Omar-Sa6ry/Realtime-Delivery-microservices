package graphql

import (
	"context"
	"strings"
	"testing"

	gql "github.com/graphql-go/graphql"
)

func newTestSchema(t *testing.T) gql.Schema {
	t.Helper()
	schema, err := buildSchema(&Handler{})
	if err != nil {
		t.Fatalf("buildSchema: %v", err)
	}
	return schema
}

func runQuery(t *testing.T, schema gql.Schema, query string) *gql.Result {
	t.Helper()
	return gql.Do(gql.Params{
		Schema:        schema,
		RequestString: query,
		Context:       context.Background(),
	})
}

func TestServiceField(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `{ _service { sdl } }`)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	service, ok := data["_service"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected _service map, got %T", data["_service"])
	}
	sdl, ok := service["sdl"].(string)
	if !ok {
		t.Fatalf("expected sdl string, got %T", service["sdl"])
	}
	if !strings.Contains(sdl, "type Query") {
		t.Errorf("SDL missing Query type: %s", sdl)
	}
	if !strings.Contains(sdl, "mediaId: String!") {
		t.Errorf("SDL missing mediaId field: %s", sdl)
	}
	if !strings.Contains(sdl, "@link") {
		t.Errorf("SDL missing federation @link directive: %s", sdl)
	}
}

func TestIntrospection(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `{ __schema { queryType { name } } }`)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	data := result.Data.(map[string]interface{})["__schema"].(map[string]interface{})
	qt := data["queryType"].(map[string]interface{})
	if qt["name"] != "Query" {
		t.Errorf("unexpected query type name: %v", qt["name"])
	}
}

func TestAuthRequired(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `{ quota { usedBytes } }`)

	if len(result.Errors) == 0 {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(result.Errors[0].Message, "x-user-id") {
		t.Errorf("unexpected error message: %v", result.Errors[0].Message)
	}
}
