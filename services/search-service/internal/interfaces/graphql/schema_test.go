package graphql

import (
	"context"
	"strings"
	"testing"

	gql "github.com/graphql-go/graphql"
)

func newTestSchema(t *testing.T) gql.Schema {
	t.Helper()
	resolver := &Resolver{}
	schema, err := buildSchema(resolver)
	if err != nil {
		t.Fatalf("buildSchema: %v", err)
	}
	return schema
}

func runQuery(t *testing.T, schema gql.Schema, query string, variables map[string]interface{}) *gql.Result {
	t.Helper()
	return gql.Do(gql.Params{
		Schema:         schema,
		RequestString:  query,
		VariableValues: variables,
		Context:        context.Background(),
	})
}

func TestServiceField(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `{ _service { sdl } }`, nil)

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
	if !strings.Contains(sdl, "searchDeliveries") {
		t.Errorf("SDL missing searchDeliveries field: %s", sdl)
	}
	if !strings.Contains(sdl, "@link") {
		t.Errorf("SDL missing federation @link directive: %s", sdl)
	}
	if !strings.Contains(sdl, "v2.3") {
		t.Errorf("SDL should use federation v2.3: %s", sdl)
	}
}

func TestIntrospection(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `{ __schema { queryType { name } } }`, nil)

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
	result := runQuery(t, schema, `{ searchDeliveries(input: { pagination: { limit: 10 } }) { items { delivery_id } } }`, nil)

	if len(result.Errors) == 0 {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(result.Errors[0].Message, "authentication required") {
		t.Errorf("unexpected error message: %v", result.Errors[0].Message)
	}
}

func TestSearchDeliveriesRequiresAuth(t *testing.T) {
	schema := newTestSchema(t)
	query := `
		query {
			searchDeliveries(input: { pagination: { limit: 10 } }) {
				items { delivery_id }
				pageInfo { hasNextPage total }
			}
		}
	`
	result := runQuery(t, schema, query, nil)

	if len(result.Errors) == 0 {
		t.Fatal("expected auth error for searchDeliveries")
	}
}

func TestSearchUsersRequiresAdmin(t *testing.T) {
	schema := newTestSchema(t)
	query := `
		query {
			searchUsers(input: { pagination: { limit: 10 } }) {
				items { id email }
				pageInfo { hasNextPage total }
			}
		}
	`
	result := runQuery(t, schema, query, nil)

	if len(result.Errors) == 0 {
		t.Fatal("expected auth error for searchUsers")
	}
}

func TestStartReindexRequiresAdmin(t *testing.T) {
	schema := newTestSchema(t)
	query := `
		mutation {
			startReindex(index: "deliveries") {
				jobId
				status
			}
		}
	`
	result := runQuery(t, schema, query, nil)

	if len(result.Errors) == 0 {
		t.Fatal("expected auth error for startReindex")
	}
}

func TestSchemaHasAllQueries(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `
		{
			__schema {
				queryType {
					fields {
						name
					}
				}
			}
		}
	`, nil)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	data := result.Data.(map[string]interface{})["__schema"].(map[string]interface{})
	queryType := data["queryType"].(map[string]interface{})
	fields := queryType["fields"].([]interface{})

	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fieldNames[f.(map[string]interface{})["name"].(string)] = true
	}

	expectedQueries := []string{
		"_service",
		"searchDeliveries",
		"searchDrivers",
		"searchMedia",
		"searchUsers",
		"autocomplete",
		"nearbyDeliveries",
		"nearbyDrivers",
		"searchHealth",
	}

	for _, q := range expectedQueries {
		if !fieldNames[q] {
			t.Errorf("missing query field: %s", q)
		}
	}
}

func TestSchemaHasMutations(t *testing.T) {
	schema := newTestSchema(t)
	result := runQuery(t, schema, `
		{
			__schema {
				mutationType {
					fields {
						name
					}
				}
			}
		}
	`, nil)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	data := result.Data.(map[string]interface{})["__schema"].(map[string]interface{})
	mutationType := data["mutationType"].(map[string]interface{})
	fields := mutationType["fields"].([]interface{})

	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fieldNames[f.(map[string]interface{})["name"].(string)] = true
	}

	if !fieldNames["startReindex"] {
		t.Error("missing mutation field: startReindex")
	}
}

func TestVariablesParsing(t *testing.T) {
	schema := newTestSchema(t)
	query := `
		query SearchDeliveries($input: DeliverySearchInput!) {
			searchDeliveries(input: $input) {
				items { delivery_id }
				pageInfo { hasNextPage total }
			}
		}
	`
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"pagination": map[string]interface{}{
				"limit": 5,
			},
		},
	}

	// This will fail auth but should parse variables correctly
	result := runQuery(t, schema, query, variables)

	// Should get auth error, not variable parsing error
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "variable") || strings.Contains(err.Message, "Variable") {
				t.Errorf("variable parsing error: %v", err.Message)
			}
		}
	}
}