package redis_test

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/redis"
)

// freshGeo clears the shared geo key before and after a test so every test
// starts from a deterministic state.
func freshGeo(t *testing.T) *goredis.Client {
	t.Helper()
	client := requireRedis(t)
	ctx := context.Background()
	if err := client.Del(ctx, geoKey).Err(); err != nil {
		t.Fatalf("cleanup geo key: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(context.Background(), geoKey).Err()
	})
	return client
}

func TestGeoStoreGEOAdd(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	tests := []struct {
		name      string
		longitude string
		latitude  string
		driverID  string
		wantErr   string
	}{
		{"accepts valid coordinates", "31.2001", "30.0444", "g-valid", ""},
		{"rejects an invalid longitude", "not-a-number", "30.0", "g-bad-lng", "invalid longitude"},
		{"rejects an invalid latitude", "31.2", "not-a-number", "g-bad-lat", "invalid latitude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.GEOAdd(ctx, tt.longitude, tt.latitude, tt.driverID)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("GEOAdd() expected an error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("GEOAdd() error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GEOAdd() error: %v", err)
			}
			if count, err := client.ZScore(ctx, geoKey, tt.driverID).Result(); err != nil || count == 0 {
				t.Errorf("driver %q missing from the geo index (score=%v, err=%v)", tt.driverID, count, err)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := store.GEOAdd(cctx, "31.2", "30.0", "g-cancelled"); err == nil {
			t.Error("GEOAdd() expected an error for a cancelled context, got nil")
		}
	})
}

func TestGeoStoreGEOSearch(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	t.Run("succeeds when drivers are in range", func(t *testing.T) {
		if err := store.GEOAdd(ctx, "31.2001", "30.0444", "g-near"); err != nil {
			t.Fatalf("GEOAdd() error: %v", err)
		}
		if err := store.GEOSearch(ctx, 31.2, 30.0, 5.0); err != nil {
			t.Errorf("GEOSearch() error: %v", err)
		}
	})

	t.Run("succeeds when no driver is in range", func(t *testing.T) {
		if err := store.GEOSearch(ctx, 0.0, 0.0, 0.001); err != nil {
			t.Errorf("GEOSearch() error: %v", err)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := store.GEOSearch(cctx, 31.2, 30.0, 5.0); err == nil {
			t.Error("GEOSearch() expected an error for a cancelled context, got nil")
		}
	})
}

func TestGeoStoreGEOSearchDrivers(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	// Roughly 1.5km and 7.4km north-east of the search origin.
	seed := []struct {
		lng, lat float64
		id       string
	}{
		{31.2100, 30.0100, "g-first"},
		{31.2500, 30.0500, "g-second"},
	}
	for _, s := range seed {
		if err := store.GEOAdd(ctx, strconv.FormatFloat(s.lng, 'f', 6, 64), strconv.FormatFloat(s.lat, 'f', 6, 64), s.id); err != nil {
			t.Fatalf("GEOAdd() error: %v", err)
		}
	}

	t.Run("returns drivers sorted by distance with computed distances", func(t *testing.T) {
		got, err := store.GEOSearchDrivers(ctx, 31.2, 30.0, 20.0)
		if err != nil {
			t.Fatalf("GEOSearchDrivers() error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("GEOSearchDrivers() returned %d drivers, want 2", len(got))
		}
		if got[0].DriverID != "g-first" || got[1].DriverID != "g-second" {
			t.Errorf("GEOSearchDrivers() order = [%s, %s], want [g-first, g-second]", got[0].DriverID, got[1].DriverID)
		}
		if got[0].DistanceMeters <= 0 {
			t.Errorf("first distance = %v, want > 0", got[0].DistanceMeters)
		}
		if got[0].DistanceMeters >= got[1].DistanceMeters {
			t.Errorf("distances are not ascending: %v, %v", got[0].DistanceMeters, got[1].DistanceMeters)
		}
		// The origin is ~7.4km from the second driver.
		if got[1].DistanceMeters < 4000 || got[1].DistanceMeters > 9000 {
			t.Errorf("second distance = %v meters, want about 7400", got[1].DistanceMeters)
		}
		if math.Abs(got[0].Longitude-31.21) > 0.01 || math.Abs(got[0].Latitude-30.01) > 0.01 {
			t.Errorf("first driver coordinates = (%v, %v), want about (31.21, 30.01)", got[0].Longitude, got[0].Latitude)
		}
	})

	t.Run("returns an empty result when no driver is in range", func(t *testing.T) {
		got, err := store.GEOSearchDrivers(ctx, -74.0, 40.7, 1.0)
		if err != nil {
			t.Fatalf("GEOSearchDrivers() error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GEOSearchDrivers() = %+v, want an empty result", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := store.GEOSearchDrivers(cctx, 31.2, 30.0, 20.0)
		if err == nil {
			t.Fatal("GEOSearchDrivers() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("GEOSearchDrivers() = %+v, want nil on error", got)
		}
	})
}

func TestGeoStoreNearbyDriversSorted(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	t.Run("returns nearby drivers sorted", func(t *testing.T) {
		if err := store.GEOAdd(ctx, "31.2000", "30.0000", "g-nearby"); err != nil {
			t.Fatalf("GEOAdd() error: %v", err)
		}
		got, err := store.NearbyDriversSorted(ctx, 31.2, 30.0, 10.0)
		if err != nil {
			t.Fatalf("NearbyDriversSorted() error: %v", err)
		}
		if len(got) != 1 || got[0].DriverID != "g-nearby" {
			t.Fatalf("NearbyDriversSorted() = %+v, want [g-nearby]", got)
		}
		if got[0].DistanceMeters < 0 || got[0].DistanceMeters > 100 {
			t.Errorf("DistanceMeters = %v, want < 100", got[0].DistanceMeters)
		}
	})

	t.Run("returns an empty result for an empty geo index", func(t *testing.T) {
		if err := client.Del(ctx, geoKey).Err(); err != nil {
			t.Fatalf("DEL: %v", err)
		}
		got, err := store.NearbyDriversSorted(ctx, 31.2, 30.0, 10.0)
		if err != nil {
			t.Fatalf("NearbyDriversSorted() error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("NearbyDriversSorted() = %+v, want an empty result", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := store.NearbyDriversSorted(cctx, 31.2, 30.0, 10.0)
		if err == nil {
			t.Fatal("NearbyDriversSorted() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("NearbyDriversSorted() = %+v, want nil on error", got)
		}
	})
}

func TestGeoStoreSetDriverLocation(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	t.Run("stores the location with latitude first", func(t *testing.T) {
		// SetDriverLocation takes (driverID, latitude, longitude).
		if err := store.SetDriverLocation(ctx, "g-set", "30.500000", "31.500000"); err != nil {
			t.Fatalf("SetDriverLocation() error: %v", err)
		}
		lat, lng, err := store.GetDriverLocation(ctx, "g-set")
		if err != nil {
			t.Fatalf("GetDriverLocation() error: %v", err)
		}
		if !approxEqual(lat, 30.5) {
			t.Errorf("latitude = %s, want about 30.5", lat)
		}
		if !approxEqual(lng, 31.5) {
			t.Errorf("longitude = %s, want about 31.5", lng)
		}
	})

	t.Run("propagates coordinate validation errors", func(t *testing.T) {
		if err := store.SetDriverLocation(ctx, "g-set-bad", "not-a-lat", "31.5"); err == nil {
			t.Error("SetDriverLocation() expected an error for an invalid latitude, got nil")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := store.SetDriverLocation(cctx, "g-set-cancelled", "30.0", "31.0"); err == nil {
			t.Error("SetDriverLocation() expected an error for a cancelled context, got nil")
		}
	})
}

func TestGeoStoreGetDriverLocation(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	t.Run("returns the stored coordinates", func(t *testing.T) {
		if err := store.GEOAdd(ctx, "31.235700", "30.046100", "g-get"); err != nil {
			t.Fatalf("GEOAdd() error: %v", err)
		}
		lat, lng, err := store.GetDriverLocation(ctx, "g-get")
		if err != nil {
			t.Fatalf("GetDriverLocation() error: %v", err)
		}
		if !approxEqual(lat, 30.0461) {
			t.Errorf("latitude = %s, want about 30.0461", lat)
		}
		if !approxEqual(lng, 31.2357) {
			t.Errorf("longitude = %s, want about 31.2357", lng)
		}
	})

	t.Run("errors when the driver has no location", func(t *testing.T) {
		// The key exists (g-get) but the member does not.
		_, _, err := store.GetDriverLocation(ctx, "g-unknown")
		if err == nil {
			t.Fatal("GetDriverLocation() expected an error for an unknown driver, got nil")
		}
		if !strings.Contains(err.Error(), "has no location") {
			t.Errorf("GetDriverLocation() error = %q, want it to contain \"has no location\"", err.Error())
		}
	})

	t.Run("errors when the geo index is empty", func(t *testing.T) {
		if err := client.Del(ctx, geoKey).Err(); err != nil {
			t.Fatalf("DEL: %v", err)
		}
		_, _, err := store.GetDriverLocation(ctx, "g-any")
		if err == nil {
			t.Fatal("GetDriverLocation() expected an error for an empty geo index, got nil")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		_, _, err := store.GetDriverLocation(cctx, "g-get")
		if err == nil {
			t.Fatal("GetDriverLocation() expected an error for a cancelled context, got nil")
		}
	})
}

func TestGeoStoreRemoveDriver(t *testing.T) {
	client := freshGeo(t)
	store := redisadapter.NewGeoStore(client)
	ctx := context.Background()

	t.Run("removes a driver from the geo index", func(t *testing.T) {
		if err := store.GEOAdd(ctx, "31.2000", "30.0000", "g-remove"); err != nil {
			t.Fatalf("GEOAdd() error: %v", err)
		}
		if err := store.RemoveDriver(ctx, "g-remove"); err != nil {
			t.Fatalf("RemoveDriver() error: %v", err)
		}
		if _, _, err := store.GetDriverLocation(ctx, "g-remove"); err == nil {
			t.Error("GetDriverLocation() expected an error after RemoveDriver(), got nil")
		}
	})

	t.Run("does not fail for an unknown driver", func(t *testing.T) {
		if err := store.RemoveDriver(ctx, "g-never-added"); err != nil {
			t.Errorf("RemoveDriver() error for an unknown driver: %v", err)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := store.RemoveDriver(cctx, "g-remove"); err == nil {
			t.Error("RemoveDriver() expected an error for a cancelled context, got nil")
		}
	})
}

// approxEqual parses a coordinate string and compares it with want within a
// small tolerance (redis rounds geo coordinates to sub-millimeter precision).
func approxEqual(got string, want float64) bool {
	v, err := strconv.ParseFloat(got, 64)
	if err != nil {
		return false
	}
	return math.Abs(v-want) < 1e-4
}
