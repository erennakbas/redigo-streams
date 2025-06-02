package strego

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
)

// MessageHandler defines the interface for message processing
type MessageHandler[T proto.Message] interface {
	Handle(ctx context.Context, message T) error
}

// MessageHandlerFunc is a function adapter for MessageHandler
type MessageHandlerFunc[T proto.Message] func(ctx context.Context, message T) error

func (f MessageHandlerFunc[T]) Handle(ctx context.Context, message T) error {
	return f(ctx, message)
}

// PublisherConfig holds configuration for the publisher
type PublisherConfig struct {
	RedisURL     string
	MaxRetries   int
	RetryBackoff time.Duration
}

// ConsumerConfig holds configuration for the consumer
type ConsumerConfig struct {
	RedisURL      string
	ConsumerGroup string
	ConsumerName  string
	BatchSize     int
	BlockTime     time.Duration
	MaxRetries    int
	RetryBackoff  time.Duration
}

// Publisher interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, stream string, message proto.Message) error
	PublishDelayed(ctx context.Context, stream string, message proto.Message, delay time.Duration) error
	Close() error
}

// Consumer interface for consuming messages
type Consumer interface {
	Subscribe(stream string, handler interface{}) error
	Start(ctx context.Context) error
	Stop() error
	Close() error
}

// StreamsClient interface for Redis Streams operations
type StreamsClient interface {
	Publisher
	Consumer
}

// ProcessingResult represents the result of message processing
type ProcessingResult struct {
	Success bool
	Error   error
	Retry   bool
}
