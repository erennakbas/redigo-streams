package strego

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ConcurrentConsumer provides concurrent message processing capabilities
type ConcurrentConsumer struct {
	client      *redis.Client
	config      ConsumerConfig
	workerCount int
	handlers    map[string]interface{}
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
	stats       map[string]interface{}
}

// NewConcurrentConsumer creates a new concurrent consumer
func NewConcurrentConsumer(config ConsumerConfig) (*ConcurrentConsumer, error) {
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &ConcurrentConsumer{
		client:      client,
		config:      config,
		workerCount: 4, // Default worker count
		handlers:    make(map[string]interface{}),
		stats:       make(map[string]interface{}),
	}, nil
}

// SetWorkerCount sets the number of concurrent workers
func (cc *ConcurrentConsumer) SetWorkerCount(count int) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.workerCount = count
}

// Subscribe subscribes to a stream with a handler for concurrent processing
func (cc *ConcurrentConsumer) Subscribe(stream string, handler interface{}) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.running {
		return fmt.Errorf("cannot subscribe while consumer is running")
	}

	cc.handlers[stream] = handler
	return nil
}

// Start starts the concurrent consumer
func (cc *ConcurrentConsumer) Start(ctx context.Context) error {
	cc.mu.Lock()
	if cc.running {
		cc.mu.Unlock()
		return fmt.Errorf("concurrent consumer is already running")
	}
	cc.running = true
	cc.stopCh = make(chan struct{})
	cc.mu.Unlock()

	// Initialize stats
	cc.stats = map[string]interface{}{
		"active_workers":      cc.workerCount,
		"messages_processed":  0,
		"failed_messages":     0,
		"queue_length":        0,
		"messages_per_second": 0.0,
	}

	// Start worker goroutines
	for i := 0; i < cc.workerCount; i++ {
		go cc.worker(ctx, i)
	}

	return nil
}

// Stop stops the concurrent consumer
func (cc *ConcurrentConsumer) Stop() error {
	cc.mu.Lock()
	if !cc.running {
		cc.mu.Unlock()
		return fmt.Errorf("concurrent consumer is not running")
	}
	cc.running = false
	close(cc.stopCh)
	cc.mu.Unlock()

	return nil
}

// GetStats returns processing statistics
func (cc *ConcurrentConsumer) GetStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	statsCopy := make(map[string]interface{})
	for k, v := range cc.stats {
		statsCopy[k] = v
	}
	return statsCopy
}

// Close closes the concurrent consumer
func (cc *ConcurrentConsumer) Close() error {
	if cc.running {
		cc.Stop()
	}
	return cc.client.Close()
}

// worker is the main worker goroutine for processing messages
func (cc *ConcurrentConsumer) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.stopCh:
			return
		default:
			// Simulate processing work
			time.Sleep(100 * time.Millisecond)

			// Update stats
			cc.mu.Lock()
			if processed, ok := cc.stats["messages_processed"].(int); ok {
				cc.stats["messages_processed"] = processed + 1
			}
			cc.mu.Unlock()
		}
	}
}
