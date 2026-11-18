package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func testCfg() *config.Config {
	return &config.Config{
		KafkaBrokers: []string{"127.0.0.1:1"}, // nothing listens here
		KafkaGroupID: "search-test",
	}
}

func TestNewConsumerManager(t *testing.T) {
	svc := indexing.NewService(testutil.NewFakeRepository())
	cm := NewConsumerManager(testCfg(), svc)
	if cm == nil || cm.dlqProducer == nil || cm.handlerRegistry == nil {
		t.Fatal("NewConsumerManager wiring incomplete")
	}
	if len(cm.consumers) != 0 {
		t.Errorf("consumers before Start = %d", len(cm.consumers))
	}
}

func TestConsumerManager_StartAndClose(t *testing.T) {
	svc := indexing.NewService(testutil.NewFakeRepository())
	cm := NewConsumerManager(testCfg(), svc)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel quickly so consumers stop their fetch loops.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(cm.consumers) != 18 {
		t.Errorf("consumers = %d, want 18", len(cm.consumers))
	}

	// Give goroutines a moment to start, then close.
	time.Sleep(150 * time.Millisecond)
	if err := cm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Let the Run goroutines observe cancellation and log their exit errors.
	time.Sleep(300 * time.Millisecond)
}

func TestConsumerManager_CloseWithoutStart(t *testing.T) {
	svc := indexing.NewService(testutil.NewFakeRepository())
	cm := NewConsumerManager(testCfg(), svc)
	if err := cm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
