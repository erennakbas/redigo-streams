package redigo

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
)

// Client wraps both Publisher and Consumer functionality
type Client struct {
	publisher          *RedisPublisher
	consumer           *RedisConsumer
	concurrentConsumer *ConcurrentConsumer
}

// DefaultPublisherConfig returns a default publisher configuration
func DefaultPublisherConfig(redisURL string) PublisherConfig {
	return PublisherConfig{
		RedisURL:     redisURL,
		MaxRetries:   3,
		RetryBackoff: time.Second,
	}
}

// DefaultConsumerConfig returns a default consumer configuration
func DefaultConsumerConfig(redisURL, consumerGroup, consumerName string) ConsumerConfig {
	return ConsumerConfig{
		RedisURL:      redisURL,
		ConsumerGroup: consumerGroup,
		ConsumerName:  consumerName,
		BatchSize:     10,
		BlockTime:     time.Second,
		MaxRetries:    3,
		RetryBackoff:  time.Second,
	}
}

// NewClient creates a new redigo client with both publisher and consumer
func NewClient(publisherConfig PublisherConfig, consumerConfig ConsumerConfig) (*Client, error) {
	publisher, err := NewPublisher(publisherConfig)
	if err != nil {
		return nil, err
	}

	consumer, err := NewConsumer(consumerConfig)
	if err != nil {
		publisher.Close()
		return nil, err
	}

	return &Client{
		publisher: publisher,
		consumer:  consumer,
	}, nil
}

// NewPublisherOnly creates a client with only publisher functionality
func NewPublisherOnly(config PublisherConfig) (*Client, error) {
	publisher, err := NewPublisher(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		publisher: publisher,
	}, nil
}

// NewConsumerOnly creates a client with only consumer functionality
func NewConsumerOnly(config ConsumerConfig) (*Client, error) {
	consumer, err := NewConsumer(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		consumer: consumer,
	}, nil
}

// NewConcurrentConsumerOnly creates a client with only concurrent consumer functionality
func NewConcurrentConsumerOnly(config ConsumerConfig) (*Client, error) {
	concurrentConsumer, err := NewConcurrentConsumer(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		concurrentConsumer: concurrentConsumer,
	}, nil
}

// Publish publishes a message to the specified stream
func (c *Client) Publish(ctx context.Context, stream string, message proto.Message) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.Publish(ctx, stream, message)
}

// PublishDelayed publishes a message with a delay
func (c *Client) PublishDelayed(ctx context.Context, stream string, message proto.Message, delay time.Duration) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishDelayed(ctx, stream, message, delay)
}

// PublishAt publishes a message at a specific time
func (c *Client) PublishAt(ctx context.Context, stream string, message proto.Message, executeAt time.Time) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishAt(ctx, stream, message, executeAt)
}

// PublishWithIdempotencyKey publishes a message with custom idempotency key to prevent duplicates
func (c *Client) PublishWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, idempotencyKey string) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishWithIdempotencyKey(ctx, stream, message, idempotencyKey)
}

// PublishWithContentDeduplication publishes with automatic content-based deduplication
func (c *Client) PublishWithContentDeduplication(ctx context.Context, stream string, message proto.Message) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishWithContentDeduplication(ctx, stream, message)
}

// SetDeduplicationConfig configures message deduplication
func (c *Client) SetDeduplicationConfig(config DeduplicationConfig) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	c.publisher.SetDeduplicationConfig(config)
	return nil
}

// GetDeduplicationStats returns deduplication statistics
func (c *Client) GetDeduplicationStats(ctx context.Context) (map[string]interface{}, error) {
	if c.publisher == nil {
		return nil, ErrPublisherNotAvailable
	}

	deduplicator := c.publisher.GetDeduplicator()
	if deduplicator == nil {
		return map[string]interface{}{"enabled": false}, nil
	}

	stats, err := deduplicator.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	stats["enabled"] = true
	return stats, nil
}

// StartDelayedScheduler starts the delayed task scheduler
func (c *Client) StartDelayedScheduler(ctx context.Context) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	c.publisher.StartScheduler(ctx)
	return nil
}

// StopDelayedScheduler stops the delayed task scheduler
func (c *Client) StopDelayedScheduler() error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	c.publisher.StopScheduler()
	return nil
}

// GetDelayedScheduler returns the delayed task scheduler
func (c *Client) GetDelayedScheduler() (*DelayedTaskScheduler, error) {
	if c.publisher == nil {
		return nil, ErrPublisherNotAvailable
	}
	scheduler := c.publisher.GetScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("delayed scheduler not available")
	}
	return scheduler, nil
}

// GetDelayedTaskStats returns delayed task statistics
func (c *Client) GetDelayedTaskStats(ctx context.Context) (map[string]interface{}, error) {
	scheduler, err := c.GetDelayedScheduler()
	if err != nil {
		return nil, err
	}
	return scheduler.GetStats(ctx)
}

// GetPendingDelayedTasks returns pending delayed tasks
func (c *Client) GetPendingDelayedTasks(ctx context.Context, limit int64) ([]*DelayedTask, error) {
	scheduler, err := c.GetDelayedScheduler()
	if err != nil {
		return nil, err
	}
	return scheduler.GetPendingTasks(ctx, limit)
}

// GetFailedDelayedTasks returns failed delayed tasks
func (c *Client) GetFailedDelayedTasks(ctx context.Context, limit int64) ([]*DelayedTask, error) {
	scheduler, err := c.GetDelayedScheduler()
	if err != nil {
		return nil, err
	}
	return scheduler.GetFailedTasks(ctx, limit)
}

// Subscribe subscribes to a stream with a handler
func (c *Client) Subscribe(stream string, handler interface{}) error {
	if c.consumer == nil {
		return ErrConsumerNotAvailable
	}
	return c.consumer.Subscribe(stream, handler)
}

// StartConsuming starts consuming messages
func (c *Client) StartConsuming(ctx context.Context) error {
	if c.consumer == nil {
		return ErrConsumerNotAvailable
	}
	return c.consumer.Start(ctx)
}

// StopConsuming stops consuming messages
func (c *Client) StopConsuming() error {
	if c.consumer == nil {
		return ErrConsumerNotAvailable
	}
	return c.consumer.Stop()
}

// EnableRecovery enables message recovery with custom configuration
func (c *Client) EnableRecovery(config RecoveryConfig) error {
	if c.consumer == nil {
		return ErrConsumerNotAvailable
	}
	c.consumer.EnableRecovery(config)
	return nil
}

// GetRecovery returns the message recovery instance
func (c *Client) GetRecovery() (*MessageRecovery, error) {
	if c.consumer == nil {
		return nil, ErrConsumerNotAvailable
	}
	return c.consumer.GetRecovery(), nil
}

// EnableConcurrentProcessing enables concurrent message processing
func (c *Client) EnableConcurrentProcessing(workerCount int) error {
	if c.concurrentConsumer == nil {
		return fmt.Errorf("concurrent consumer not available")
	}
	c.concurrentConsumer.SetWorkerCount(workerCount)
	return nil
}

// GetProcessingStats returns concurrent processing statistics
func (c *Client) GetProcessingStats() (map[string]interface{}, error) {
	if c.concurrentConsumer == nil {
		return nil, fmt.Errorf("concurrent consumer not available")
	}
	return c.concurrentConsumer.GetStats(), nil
}

// SubscribeConcurrent subscribes to a stream with concurrent processing
func (c *Client) SubscribeConcurrent(stream string, handler interface{}) error {
	if c.concurrentConsumer == nil {
		return fmt.Errorf("concurrent consumer not available")
	}
	return c.concurrentConsumer.Subscribe(stream, handler)
}

// StartConcurrentConsuming starts concurrent consuming
func (c *Client) StartConcurrentConsuming(ctx context.Context) error {
	if c.concurrentConsumer == nil {
		return fmt.Errorf("concurrent consumer not available")
	}
	return c.concurrentConsumer.Start(ctx)
}

// StopConcurrentConsuming stops concurrent consuming
func (c *Client) StopConcurrentConsuming() error {
	if c.concurrentConsumer == nil {
		return fmt.Errorf("concurrent consumer not available")
	}
	return c.concurrentConsumer.Stop()
}

// Close closes both publisher and consumer
func (c *Client) Close() error {
	var publisherErr, consumerErr, concurrentConsumerErr error

	if c.publisher != nil {
		publisherErr = c.publisher.Close()
	}

	if c.consumer != nil {
		consumerErr = c.consumer.Close()
	}

	if c.concurrentConsumer != nil {
		concurrentConsumerErr = c.concurrentConsumer.Close()
	}

	// Return the first error encountered
	if publisherErr != nil {
		return publisherErr
	}
	if consumerErr != nil {
		return consumerErr
	}
	return concurrentConsumerErr
}

// PublishDelayedWithIdempotencyKey publishes a message with delay and custom idempotency key
func (c *Client) PublishDelayedWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, delay time.Duration, idempotencyKey string) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishDelayedWithIdempotencyKey(ctx, stream, message, delay, idempotencyKey)
}

// PublishAtWithIdempotencyKey publishes a message at specific time with custom idempotency key
func (c *Client) PublishAtWithIdempotencyKey(ctx context.Context, stream string, message proto.Message, executeAt time.Time, idempotencyKey string) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishAtWithIdempotencyKey(ctx, stream, message, executeAt, idempotencyKey)
}

// PublishDelayedWithContentDeduplication publishes a message with delay and automatic content-based deduplication
func (c *Client) PublishDelayedWithContentDeduplication(ctx context.Context, stream string, message proto.Message, delay time.Duration) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishDelayedWithContentDeduplication(ctx, stream, message, delay)
}

// PublishAtWithContentDeduplication publishes a message at specific time with automatic content-based deduplication
func (c *Client) PublishAtWithContentDeduplication(ctx context.Context, stream string, message proto.Message, executeAt time.Time) error {
	if c.publisher == nil {
		return ErrPublisherNotAvailable
	}
	return c.publisher.PublishAtWithContentDeduplication(ctx, stream, message, executeAt)
}
