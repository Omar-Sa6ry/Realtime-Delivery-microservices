package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
)

type mockPlatformOverviewService struct {
	result *ports.PlatformOverview
	err    error
}

func (m *mockPlatformOverviewService) GetPlatformOverview(ctx context.Context, r ports.TimeRange, scope string) (*ports.PlatformOverview, error) {
	return m.result, m.err
}

type mockDeliveryAnalyticsService struct {
	result *ports.DeliveryAnalytics
	err    error
}

func (m *mockDeliveryAnalyticsService) GetDeliveryAnalytics(ctx context.Context, f ports.DeliveryAnalyticsFilter, scope string) (*ports.DeliveryAnalytics, error) {
	return m.result, m.err
}

type mockDriverAnalyticsService struct {
	driverResult    *ports.DriverAnalytics
	topDriversResult *ports.DriverAnalyticsPage
	err             error
}

func (m *mockDriverAnalyticsService) GetDriverAnalytics(ctx context.Context, f ports.DriverAnalyticsFilter, scope string) (*ports.DriverAnalytics, error) {
	return m.driverResult, m.err
}

func (m *mockDriverAnalyticsService) GetTopDrivers(ctx context.Context, r ports.TimeRange, limit int, scope string) (*ports.DriverAnalyticsPage, error) {
	return m.topDriversResult, m.err
}

type mockPaymentAnalyticsService struct {
	result *ports.PaymentAnalytics
	err    error
}

func (m *mockPaymentAnalyticsService) GetPaymentAnalytics(ctx context.Context, f ports.PaymentAnalyticsFilter, scope string) (*ports.PaymentAnalytics, error) {
	return m.result, m.err
}

type mockQueryRepo struct {
	ports.AnalyticsQueryRepository
	rawEventsResult      *ports.RawEventsPage
	dataQualityResult    *ports.DataQualityIssuesPage
}

func (m *mockQueryRepo) RawEvents(ctx context.Context, page, limit int, eventType string, from, to *time.Time) (*ports.RawEventsPage, error) {
	return m.rawEventsResult, nil
}

func (m *mockQueryRepo) DataQualityIssues(ctx context.Context, page, limit int, severity string) (*ports.DataQualityIssuesPage, error) {
	return m.dataQualityResult, nil
}

func setupResolver() *Resolver {
	mockRepo := testutil.NewMockAnalyticsQueryRepository()
	mockCache := testutil.NewMockCacheRepository()

	mockRepo.SetPlatformOverview(&ports.PlatformOverview{
		TotalDeliveries:          100,
		CompletedDeliveries:      80,
		CancelledDeliveries:      10,
		FailedDeliveries:         10,
		CompletionRate:           0.8,
		AverageDeliveryDurationS: 1800,
		PaymentCapturedAmount:    "8000.00",
		RefundedAmount:           "500.00",
		DriverAcceptanceRate:     0.85,
		DataAsOf:                 time.Now().UTC(),
	})

	mockRepo.SetDeliveryAnalytics(&ports.DeliveryAnalytics{
		Total:                  100,
		Completed:              80,
		Cancelled:              10,
		Failed:                 10,
		CompletionRate:         0.8,
		AverageDurationSeconds: 1800,
		DataAsOf:               time.Now().UTC(),
	})

	mockRepo.SetDriverAnalytics(&ports.DriverAnalytics{
		DriverID:             "driver-1",
		Offers:               50,
		Accepted:             40,
		Rejected:             5,
		Expired:              5,
		AcceptanceRate:       0.8,
		AverageResponseTimeMs: 120000,
		CompletedDeliveries:  35,
		DataAsOf:             time.Now().UTC(),
	})

	mockRepo.SetTopDriversPage(&ports.DriverAnalyticsPage{
		Pagination: ports.Pagination{TotalItems: 10, CurrentPage: 1},
		Items: []ports.DriverAnalytics{
			{DriverID: "driver-1", Offers: 50, Accepted: 40},
		},
	})

	mockRepo.SetPaymentAnalytics(&ports.PaymentAnalytics{
		AuthorizationCount:       100,
		AuthorizationSuccessRate: 0.9,
		CaptureCount:             85,
		CapturedAmount:           "8500.00",
		RefundCount:              5,
		RefundedAmount:           "250.00",
		RefundRate:               0.05,
		DataAsOf:                 time.Now().UTC(),
	})

	mockRepo.SetRawEventsPage(&ports.RawEventsPage{
		Pagination: ports.Pagination{TotalItems: 5, CurrentPage: 1},
		Items: []*domain.RawEventLanding{
			testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1"),
		},
	})

	mockRepo.SetDataQualityIssuesPage(&ports.DataQualityIssuesPage{
		Pagination: ports.Pagination{TotalItems: 2, CurrentPage: 1},
		Items: []*domain.DataQualityIssue{
			testutil.NewTestDataQualityIssue("evt-1", "negative_duration"),
		},
	})

	platformSvc := analytics.NewPlatformOverviewService(mockRepo, mockCache, 5*time.Minute)
	deliverySvc := analytics.NewDeliveryAnalyticsService(mockRepo, mockCache, 5*time.Minute)
	driverSvc := analytics.NewDriverAnalyticsService(mockRepo, mockCache, 5*time.Minute)
	paymentSvc := analytics.NewPaymentAnalyticsService(mockRepo, mockCache, 5*time.Minute)

	return NewResolver(platformSvc, deliverySvc, driverSvc, paymentSvc, mockRepo)
}

func TestHealthLiveHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	HealthLiveHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, float64(200), resp["statusCode"])
	assert.Equal(t, "analytics-service", resp["data"].(map[string]any)["name"])
}

func TestHealthReadyHandler_Success(t *testing.T) {
	ping := func(ctx context.Context) error { return nil }
	handler := HealthReadyHandler(ping)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp["success"].(bool))
}

func TestHealthReadyHandler_Failure(t *testing.T) {
	ping := func(ctx context.Context) error { return errors.New("ping error") }
	handler := HealthReadyHandler(ping)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp["success"].(bool))
}

func TestLanguageMiddleware(t *testing.T) {
	var capturedLang string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLang = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := LanguageMiddleware(next)

	tests := []struct {
		name       string
		header     string
		expected   string
	}{
		{"x-lang header", "ar", "ar"},
		{"X-Lang header", "en", "en"},
		{"query param", "ar", "ar"},
		{"accept-language", "ar", "ar"},
		{"default", "", "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?lang="+tc.header, nil)
			if tc.header != "" && tc.name != "query param" {
				req.Header.Set("x-lang", tc.header)
			}
			if tc.name == "accept-language" {
				req.Header.Set("Accept-Language", tc.header)
			}
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
			assert.Equal(t, tc.expected, capturedLang)
		})
	}
}

func TestDataLoaderMiddleware(t *testing.T) {
	var capturedLoaders bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loaders := GetLoaders(r.Context())
		capturedLoaders = loaders != nil
		w.WriteHeader(http.StatusOK)
	})

	middleware := DataLoaderMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	assert.True(t, capturedLoaders)
}

func TestGraphQLHandler_ServiceInfo(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ analyticsServiceInfo { name version status } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	info := data["analyticsServiceInfo"].(map[string]any)
	innerData := info["data"].(map[string]any)
	assert.Equal(t, "analytics-service", innerData["name"])
	assert.Equal(t, "1.0.0", innerData["version"])
	assert.Equal(t, "healthy", innerData["status"])
}

func TestGraphQLHandler_PlatformOverview(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		platformOverview(range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}) {
			totalDeliveries
			completedDeliveries
			completionRate
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	overview := data["platformOverview"].(map[string]any)
	innerOverview := overview["data"].(map[string]any)
	assert.Equal(t, float64(100), innerOverview["totalDeliveries"])
	assert.Equal(t, float64(80), innerOverview["completedDeliveries"])
	assert.Equal(t, float64(0.8), innerOverview["completionRate"])
}

func TestGraphQLHandler_DeliveryAnalytics(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		deliveryAnalytics(filter: {range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}, cityId: "city-1"}) {
			total
			completed
			completionRate
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	analytics := data["deliveryAnalytics"].(map[string]any)
	innerAnalytics := analytics["data"].(map[string]any)
	assert.Equal(t, float64(100), innerAnalytics["total"])
	assert.Equal(t, float64(80), innerAnalytics["completed"])
}

func TestGraphQLHandler_DriverAnalytics(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		driverAnalytics(filter: {range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}, driverId: "driver-1"}) {
			driverId
			offers
			accepted
			acceptanceRate
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	analytics := data["driverAnalytics"].(map[string]any)
	innerAnalytics := analytics["data"].(map[string]any)
	assert.Equal(t, "driver-1", innerAnalytics["driverId"])
	assert.Equal(t, float64(50), innerAnalytics["offers"])
}

func TestGraphQLHandler_TopDrivers(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		topDrivers(range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}, limit: 10) {
			items { driverId offers }
			paginationInfo { totalItems }
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	topDrivers := data["topDrivers"].(map[string]any)
	innerData := topDrivers["data"].(map[string]any)
	items := innerData["items"].([]any)
	assert.Len(t, items, 1)
	assert.Equal(t, "driver-1", items[0].(map[string]any)["driverId"])
}

func TestGraphQLHandler_PaymentAnalytics(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		paymentAnalytics(filter: {range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}, provider: STRIPE}) {
			authorizationCount
			capturedAmount
			refundRate
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	analytics := data["paymentAnalytics"].(map[string]any)
	innerAnalytics := analytics["data"].(map[string]any)
	assert.Equal(t, float64(100), innerAnalytics["authorizationCount"])
	assert.Equal(t, "8500.00", innerAnalytics["capturedAmount"])
	assert.Equal(t, float64(0.05), innerAnalytics["refundRate"])
}

func TestGraphQLHandler_RawAnalyticsEvents(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		rawAnalyticsEvents(page: 1, limit: 10, eventType: DELIVERY_CREATED) {
			items { eventId eventType }
			paginationInfo { totalItems }
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	rawEvents := data["rawAnalyticsEvents"].(map[string]any)
	innerData := rawEvents["data"].(map[string]any)
	items := innerData["items"].([]any)
	assert.Len(t, items, 1)
}

func TestGraphQLHandler_DataQualityIssues(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	query := `{
		dataQualityIssues(page: 1, limit: 10, severity: ERROR) {
			items { issueId issueType }
			paginationInfo { totalItems }
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":`+jsonMarshal(query)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	issues := data["dataQualityIssues"].(map[string]any)
	innerData := issues["data"].(map[string]any)
	items := innerData["items"].([]any)
	assert.Len(t, items, 1)
}

func TestGraphQLHandler_Unauthorized(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	// Use a query that doesn't match any known operation to test role check
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ unknownOperation }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "user")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	errors := resp["errors"].([]any)
	assert.Len(t, errors, 1)
	assert.Equal(t, "FORBIDDEN", errors[0].(map[string]any)["extensions"].(map[string]any)["code"])
}

func TestGraphQLHandler_Introspection(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	// _service introspection
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ _service { sdl } }"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sdl")

	// __typename
	req = httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ __typename }"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Query")

	// __schema
	req = httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ __schema { types { name } } }"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "__schema")
}

func TestGraphQLHandler_InvalidJSON(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphQLHandler_NotPost(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Analytics GraphQL. Use POST with query.")
}

func TestGraphQLHandler_UnknownOperation(t *testing.T) {
	resolver := setupResolver()
	handler := GraphQLHandler(resolver)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ unknownField }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	errors := resp["errors"].([]any)
	assert.Len(t, errors, 1)
	assert.Equal(t, "operation not recognized", errors[0].(map[string]any)["message"])
}

func jsonMarshal(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "value", resp["key"])
}