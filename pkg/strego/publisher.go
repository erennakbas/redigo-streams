package strego

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/erennakbas/strego/pkg/proto"
)

// RedisPublisher implements the Publisher interface using Redis Streams
type RedisPublisher struct {
	client       *redis.Client
	config       PublisherConfig
	mu           sync.RWMutex
	closed       bool
	scheduler    *DelayedTaskScheduler
	deduplicator *MessageDeduplicator
}

// NewPublisher creates a new Redis publisher
func NewPublisher(config PublisherConfig) (*RedisPublisher, error) {
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

	publisher := &RedisPublisher{
		client: client,
		config: config,
	}

	// Initialize delayed task scheduler
	schedulerConfig := DefaultDelayedSchedulerConfig()
	publisher.scheduler = NewDelayedTaskScheduler(client, publisher, schedulerConfig)

	// Initialize message deduplicator
	dedupConfig := DefaultDeduplicationConfig()
	publisher.deduplicator = NewMessageDeduplicator(client, dedupConfig)

	return publisher, nil
}

// SetDeduplicationConfig allows customizing deduplication settings
func (p *RedisPublisher) SetDeduplicationConfig(config DeduplicationConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config.Enabled {
		p.deduplicator = NewMessageDeduplicator(p.client, config)
	} else {
		p.deduplicator = nil
	}
}

// GetDeduplicator returns the message deduplicator
func (p *RedisPublisher) GetDeduplicator() *MessageDeduplicator {
	return p.deduplicator
}

// PublishWithIdempotencyKey publishes a message with custom idempotency key
func (p *RedisPublisher) PublishWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, idempotencyKey string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, idempotencyKey, generateMessageID())
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			// Get info about the original message
			originalID, processedAt, err := p.deduplicator.GetProcessedInfo(ctx, idempotencyKey)
			if err == nil {
				return fmt.Errorf("duplicate message detected: original processed at %v with ID %s", processedAt, originalID)
			}
			return fmt.Errorf("duplicate message detected")
		}
	}

	return p.publishMessage(ctx, stream, message, nil)
}

// PublishWithContentDeduplication publishes with automatic content-based deduplication
func (p *RedisPublisher) PublishWithContentDeduplication(ctx context.Context, stream string, message proto.Message) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		contentHash := GenerateContentHash(stream, message)
		messageID := generateMessageID()

		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, contentHash, messageID)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			return fmt.Errorf("duplicate message content detected for stream %s", stream)
		}
	}

	return p.publishMessage(ctx, stream, message, nil)
}

// StartScheduler starts the delayed task scheduler
func (p *RedisPublisher) StartScheduler(ctx context.Context) {
	if p.scheduler != nil {
		p.scheduler.Start(ctx)
	}
}

// StopScheduler stops the delayed task scheduler
func (p *RedisPublisher) StopScheduler() {
	if p.scheduler != nil {
		p.scheduler.Stop()
	}
}

// GetScheduler returns the delayed task scheduler
func (p *RedisPublisher) GetScheduler() *DelayedTaskScheduler {
	return p.scheduler
}

// Publish sends a message to the specified stream
func (p *RedisPublisher) Publish(ctx context.Context, stream string, message proto.Message) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	return p.publishMessage(ctx, stream, message, nil)
}

// PublishDelayed schedules a message to be published after the specified delay
func (p *RedisPublisher) PublishDelayed(ctx context.Context, stream string, message proto.Message, delay time.Duration) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	executeAt := time.Now().Add(delay)
	return p.scheduler.ScheduleTask(ctx, stream, message, executeAt)
}

// PublishAt schedules a message to be published at a specific time
func (p *RedisPublisher) PublishAt(ctx context.Context, stream string, message proto.Message, executeAt time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	return p.scheduler.ScheduleTask(ctx, stream, message, executeAt)
}

// PublishDelayedWithIdempotencyKey schedules a message with custom idempotency key to be published after the specified delay
func (p *RedisPublisher) PublishDelayedWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, delay time.Duration, idempotencyKey string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, idempotencyKey, generateMessageID())
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			// Get info about the original message
			originalID, processedAt, err := p.deduplicator.GetProcessedInfo(ctx, idempotencyKey)
			if err == nil {
				return fmt.Errorf("duplicate delayed message detected: original scheduled at %v with ID %s", processedAt, originalID)
			}
			return fmt.Errorf("duplicate delayed message detected")
		}
	}

	executeAt := time.Now().Add(delay)
	return p.scheduler.ScheduleTaskWithIdempotency(ctx, stream, message, executeAt, idempotencyKey)
}

// PublishAtWithIdempotencyKey schedules a message with custom idempotency key to be published at a specific time
func (p *RedisPublisher) PublishAtWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, executeAt time.Time, idempotencyKey string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, idempotencyKey, generateMessageID())
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			// Get info about the original message
			originalID, processedAt, err := p.deduplicator.GetProcessedInfo(ctx, idempotencyKey)
			if err == nil {
				return fmt.Errorf("duplicate scheduled message detected: original scheduled at %v with ID %s", processedAt, originalID)
			}
			return fmt.Errorf("duplicate scheduled message detected")
		}
	}

	return p.scheduler.ScheduleTaskWithIdempotency(ctx, stream, message, executeAt, idempotencyKey)
}

// PublishDelayedWithContentDeduplication schedules a message with automatic content-based deduplication after the specified delay
func (p *RedisPublisher) PublishDelayedWithContentDeduplication(ctx context.Context, stream string, message proto.Message, delay time.Duration) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	// Generate content hash for deduplication
	contentHash := GenerateContentHash(stream, message)

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		messageID := generateMessageID()

		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, contentHash, messageID)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			return fmt.Errorf("duplicate delayed message content detected for stream %s", stream)
		}
	}

	executeAt := time.Now().Add(delay)
	return p.scheduler.ScheduleTaskWithIdempotency(ctx, stream, message, executeAt, contentHash)
}

// PublishAtWithContentDeduplication schedules a message with automatic content-based deduplication at a specific time
func (p *RedisPublisher) PublishAtWithContentDeduplication(ctx context.Context, stream string, message proto.Message, executeAt time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	if p.scheduler == nil {
		return fmt.Errorf("delayed task scheduler not available")
	}

	// Generate content hash for deduplication
	contentHash := GenerateContentHash(stream, message)

	// Check for duplicate if deduplicator is enabled
	if p.deduplicator != nil {
		messageID := generateMessageID()

		isDuplicate, err := p.deduplicator.CheckAndMark(ctx, contentHash, messageID)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate: %w", err)
		}

		if isDuplicate {
			return fmt.Errorf("duplicate scheduled message content detected for stream %s", stream)
		}
	}

	return p.scheduler.ScheduleTaskWithIdempotency(ctx, stream, message, executeAt, contentHash)
}

// publishMessage is the internal method to publish a message
func (p *RedisPublisher) publishMessage(ctx context.Context, stream string, message proto.Message, scheduledAt *time.Time) error {
	// Convert protobuf message to Any
	payloadAny, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("failed to create Any from message: %w", err)
	}

	// Create stream message
	streamMsg := &pb.StreamMessage{
		Id:          generateMessageID(),
		Stream:      stream,
		MessageType: string(message.ProtoReflect().Descriptor().FullName()),
		Payload:     payloadAny,
		CreatedAt:   timestamppb.Now(),
		Metadata:    make(map[string]string),
		RetryCount:  0,
		MaxRetries:  int32(p.config.MaxRetries),
	}

	if scheduledAt != nil {
		streamMsg.ScheduledAt = timestamppb.New(*scheduledAt)
		streamMsg.Metadata["scheduled"] = "true"
		streamMsg.Metadata["scheduled_for"] = scheduledAt.Format(time.RFC3339)
	}

	// Serialize the stream message
	data, err := proto.Marshal(streamMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal stream message: %w", err)
	}

	// Publish to Redis Stream
	argsValues := map[string]interface{}{
		"data":    data,
		"type":    streamMsg.MessageType,
		"created": streamMsg.CreatedAt.AsTime().Unix(),
	}

	if scheduledAt != nil {
		argsValues["scheduled_at"] = scheduledAt.Unix()
		argsValues["scheduled"] = "true"
	}

	args := &redis.XAddArgs{
		Stream: stream,
		Values: argsValues,
	}

	if err := p.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("failed to publish message to stream %s: %w", stream, err)
	}

	return nil
}

// PublishMessageDirectly is used by the scheduler to publish messages directly
func (p *RedisPublisher) PublishMessageDirectly(ctx context.Context, stream string, message proto.Message, scheduledAt time.Time) error {
	return p.publishMessage(ctx, stream, message, &scheduledAt)
}

// Close closes the publisher and cleanup resources
func (p *RedisPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Stop delayed task scheduler
	if p.scheduler != nil {
		p.scheduler.Stop()
	}

	return p.client.Close()
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000)
}
