package strego

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// RedisConsumer implements the Consumer interface using Redis Streams
type RedisConsumer struct {
	client   *redis.Client
	config   ConsumerConfig
	handlers map[string]reflect.Value
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
	recovery *MessageRecovery
	streams  []string // Track subscribed streams for recovery
}

// NewConsumer creates a new Redis consumer
func NewConsumer(config ConsumerConfig) (*RedisConsumer, error) {
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

	return &RedisConsumer{
		client:   client,
		config:   config,
		handlers: make(map[string]reflect.Value),
		stopCh:   make(chan struct{}),
	}, nil
}

// Subscribe registers a handler for messages from the specified stream
func (c *RedisConsumer) Subscribe(stream string, handler interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("cannot subscribe while consumer is running")
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	handlerType := handlerValue.Type()
	if handlerType.NumIn() != 2 || handlerType.NumOut() != 1 {
		return fmt.Errorf("handler must have signature func(context.Context, T) error")
	}

	if handlerType.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return fmt.Errorf("first parameter must be context.Context")
	}

	if handlerType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("return type must be error")
	}

	c.handlers[stream] = handlerValue

	// Track streams for recovery
	streamExists := false
	for _, s := range c.streams {
		if s == stream {
			streamExists = true
			break
		}
	}
	if !streamExists {
		c.streams = append(c.streams, stream)
	}

	return nil
}

// Start begins consuming messages from subscribed streams
func (c *RedisConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("consumer is already running")
	}

	if len(c.handlers) == 0 {
		return fmt.Errorf("no handlers registered")
	}

	c.running = true

	// Create consumer group for each stream if it doesn't exist
	for stream := range c.handlers {
		if err := c.createConsumerGroup(ctx, stream); err != nil {
			return fmt.Errorf("failed to create consumer group for stream %s: %w", stream, err)
		}
	}

	// Start consuming from each stream
	for stream, handler := range c.handlers {
		c.wg.Add(1)
		go c.consumeStream(ctx, stream, handler)
	}

	// Start recovery if enabled
	if c.recovery != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			fmt.Printf("🔄 Starting message recovery for streams: %v\n", c.streams)
			c.recovery.StartRecovery(ctx, c.streams, c)
		}()
	}

	return nil
}

// Stop stops the consumer
func (c *RedisConsumer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false
	close(c.stopCh)
	c.wg.Wait()

	return nil
}

// Close closes the consumer and cleanup resources
func (c *RedisConsumer) Close() error {
	if err := c.Stop(); err != nil {
		return err
	}
	return c.client.Close()
}

// createConsumerGroup creates a consumer group for the stream if it doesn't exist
func (c *RedisConsumer) createConsumerGroup(ctx context.Context, stream string) error {
	err := c.client.XGroupCreateMkStream(ctx, stream, c.config.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}

	return nil
}

// consumeStream consumes messages from a specific stream
func (c *RedisConsumer) consumeStream(ctx context.Context, stream string, handler reflect.Value) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		args := &redis.XReadGroupArgs{
			Group:    c.config.ConsumerGroup,
			Consumer: c.config.ConsumerName,
			Streams:  []string{stream, ">"},
			Count:    int64(c.config.BatchSize),
			Block:    c.config.BlockTime,
		}

		result, err := c.client.XReadGroup(ctx, args).Result()
		if err != nil {
			if err == redis.Nil {
				continue // No messages available
			}
			fmt.Printf("Error reading from stream %s: %v\n", stream, err)
			time.Sleep(c.config.RetryBackoff)
			continue
		}

		for _, streamResult := range result {
			for _, message := range streamResult.Messages {
				if err := c.processMessage(ctx, stream, message, handler); err != nil {
					fmt.Printf("Error processing message %s: %v\n", message.ID, err)
				}
			}
		}
	}
}

// ProcessMessage implements MessageProcessor interface for recovery
func (c *RedisConsumer) ProcessMessage(ctx context.Context, stream string, message redis.XMessage) error {
	c.mu.RLock()
	handler, exists := c.handlers[stream]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for stream %s", stream)
	}

	fmt.Printf("🔄 Recovery processing message %s from stream %s\n", message.ID, stream)
	return c.processMessageForRecovery(ctx, stream, message, handler)
}

// processMessageForRecovery processes a message for recovery (without ACK - handled by recovery system)
func (c *RedisConsumer) processMessageForRecovery(ctx context.Context, stream string, message redis.XMessage, handler reflect.Value) error {
	// Extract message data
	dataBytes, ok := message.Values["data"].(string)
	if !ok {
		return fmt.Errorf("message data not found or invalid type")
	}

	// Deserialize stream message
	var streamMsg StreamMessage
	if err := proto.Unmarshal([]byte(dataBytes), &streamMsg); err != nil {
		return fmt.Errorf("failed to unmarshal stream message: %w", err)
	}

	// Get the expected message type from the handler
	handlerType := handler.Type()
	expectedType := handlerType.In(1)

	// Create a new instance of the expected type
	msgValue := reflect.New(expectedType.Elem()).Interface()
	protoMsg, ok := msgValue.(proto.Message)
	if !ok {
		return fmt.Errorf("handler parameter is not a protobuf message")
	}

	// Unmarshal the payload
	if err := anypb.UnmarshalTo(streamMsg.Payload, protoMsg, proto.UnmarshalOptions{}); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Call the handler
	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(protoMsg),
	}

	results := handler.Call(args)
	if len(results) > 0 && !results[0].IsNil() {
		err := results[0].Interface().(error)
		fmt.Printf("❌ Recovery message processing failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ Recovery message %s processed successfully\n", message.ID)
	// Don't ACK here - let the recovery system handle it
	return nil
}

// processMessage processes a single message (for normal consumption)
func (c *RedisConsumer) processMessage(ctx context.Context, stream string, message redis.XMessage, handler reflect.Value) error {
	// Extract message data
	dataBytes, ok := message.Values["data"].(string)
	if !ok {
		return fmt.Errorf("message data not found or invalid type")
	}

	// Deserialize stream message
	var streamMsg StreamMessage
	if err := proto.Unmarshal([]byte(dataBytes), &streamMsg); err != nil {
		return fmt.Errorf("failed to unmarshal stream message: %w", err)
	}

	// Get the expected message type from the handler
	handlerType := handler.Type()
	expectedType := handlerType.In(1)

	// Create a new instance of the expected type
	msgValue := reflect.New(expectedType.Elem()).Interface()
	protoMsg, ok := msgValue.(proto.Message)
	if !ok {
		return fmt.Errorf("handler parameter is not a protobuf message")
	}

	// Unmarshal the payload
	if err := anypb.UnmarshalTo(streamMsg.Payload, protoMsg, proto.UnmarshalOptions{}); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Call the handler
	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(protoMsg),
	}

	results := handler.Call(args)
	if len(results) > 0 && !results[0].IsNil() {
		err := results[0].Interface().(error)
		fmt.Printf("❌ Normal message processing failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ Normal message %s processed successfully\n", message.ID)
	// ACK the message for normal processing
	return c.client.XAck(ctx, stream, c.config.ConsumerGroup, message.ID).Err()
}

// EnableRecovery enables message recovery with custom configuration
func (c *RedisConsumer) EnableRecovery(config RecoveryConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.recovery = NewMessageRecoveryWithConfig(c.client, c.config.ConsumerGroup, c.config.ConsumerName, config)
	fmt.Printf("🔄 Message recovery enabled with config: IdleTime=%v, ClaimInterval=%v, MaxRetries=%d\n",
		config.IdleTime, config.ClaimInterval, config.MaxRetries)
}

// GetRecovery returns the message recovery instance
func (c *RedisConsumer) GetRecovery() *MessageRecovery {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.recovery
}
