# Redigo Streams

A type-safe, thread-safe Redis Streams based task queue and messaging library for Go with built-in message recovery and retry mechanisms.

## Features

- 🚀 **Redis Streams** based messaging
- 🔒 **Thread-safe** operations
- 📝 **Type-safe** messages using Protocol Buffers
- ⏰ **Delayed task scheduling** with Redis Sorted Sets
- 🔄 **Message recovery** and retry mechanisms
- 💀 **Dead letter queue** support
- 🎯 **Easy to use** API
- 📊 **Publisher/Consumer** pattern
- 🔁 **Automatic message claiming** for failed consumers
- 📈 **Configurable retry logic**
- ⚡ **Concurrent message processing** with worker pools
- 🛡️ **Multiple consumer safety** guaranteed
- 📅 **Scheduler with retry and failure handling**

## Thread Safety & Concurrent Processing

### Redis Streams Consumer Groups Guarantee

Redis Streams Consumer Groups provide **built-in thread safety** at the Redis level:

- ✅ **Each message is delivered to only one consumer** within a consumer group
- ✅ **XREADGROUP is atomic** - no race conditions between consumers
- ✅ **Multiple consumer instances** can safely run in parallel
- ✅ **Pending message tracking** per consumer
- ✅ **Automatic load balancing** across consumers

### Application Level Thread Safety

Our implementation provides additional safety layers:

1. **Mutex Protection**: All consumer state changes are protected by RWMutex
2. **Goroutine Safety**: Internal operations are goroutine-safe
3. **Worker Pool**: Concurrent message processing with configurable workers
4. **Message Isolation**: Each message is processed independently

### Multiple Consumer Scenarios

#### Scenario 1: Multiple Consumer Instances (Recommended)

```go
// Consumer 1 (e.g., service instance 1)
consumer1 := redigo.NewConsumer(redigo.ConsumerConfig{
    ConsumerGroup: "user-service",     // Same group
    ConsumerName:  "instance-1",       // Different name
    // ...
})

// Consumer 2 (e.g., service instance 2)
consumer2 := redigo.NewConsumer(redigo.ConsumerConfig{
    ConsumerGroup: "user-service",     // Same group
    ConsumerName:  "instance-2",       // Different name
    // ...
})
```

**Result**: Messages are automatically distributed between consumers. If one fails, the other can claim its pending messages.

#### Scenario 2: Concurrent Processing Within One Consumer

```go
// Single consumer with worker pool
consumer := redigo.NewConcurrentConsumer(config)
consumer.SetWorkerCount(8) // 8 concurrent workers

consumer.Subscribe("events", func(ctx context.Context, event *MyEvent) error {
    workerID := ctx.Value("worker_id") // Know which worker is processing
    // Process concurrently...
    return nil
})
```

**Result**: One consumer, multiple workers processing messages concurrently.

## Concurrent Processing

### Basic Concurrent Consumer

```go
// Create concurrent consumer
config := redigo.DefaultConsumerConfig("redis://localhost:6379", "my-group", "worker-1")
consumer, err := redigo.NewConcurrentConsumerOnly(config)

// Configure worker pool
err = consumer.EnableConcurrentProcessing(8) // 8 workers

// Subscribe with concurrent processing
consumer.SubscribeConcurrent("events", func(ctx context.Context, event *MyEvent) error {
    workerID := ctx.Value("worker_id").(int)
    attempt := ctx.Value("attempt").(int)

    fmt.Printf("Worker %d processing event (attempt %d)\n", workerID, attempt)

    // Your processing logic here...
    return nil
})

// Start processing
consumer.StartConcurrentConsuming(ctx)
```

### Worker Pool Features

- **Configurable Worker Count**: Set optimal number of concurrent workers
- **Task Queue**: Buffered channel for message distribution
- **Worker Context**: Each worker gets unique ID and attempt count
- **Automatic Retry**: Failed messages are automatically retried with backoff
- **Statistics**: Real-time processing stats
- **Graceful Shutdown**: Proper worker cleanup on stop

### Processing Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────────┐
│   Redis     │───▶│  Consumer   │───▶│   Task Queue    │
│   Stream    │    │   Reader    │    │   (Buffered)    │
└─────────────┘    └─────────────┘    └─────────────────┘
                                               │
                                               ▼
                    ┌──────────────────────────────────────┐
                    │            Worker Pool               │
                    │  ┌─────────┐ ┌─────────┐ ┌─────────┐ │
                    │  │Worker 1 │ │Worker 2 │ │Worker N │ │
                    │  └─────────┘ └─────────┘ └─────────┘ │
                    └──────────────────────────────────────┘
                                               │
                                               ▼
                                    ┌─────────────────┐
                                    │   Handler       │
                                    │   Execution     │
                                    └─────────────────┘
```

### Statistics Monitoring

```go
stats, err := consumer.GetProcessingStats()
fmt.Printf("Stats: %+v\n", stats)

// Output:
// {
//   "running": true,
//   "worker_count": 8,
//   "task_queue_len": 3,
//   "task_queue_cap": 20,
//   "handlers": 2
// }
```

## Delayed Task Scheduling

The library includes a sophisticated delayed task scheduling system using Redis Sorted Sets:

### How Delayed Tasks Work

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │───▶│ Delayed     │───▶│   Redis     │
│ PublishDelayed  │ │ Scheduler   │    │ Sorted Set  │
└─────────────┘    └─────────────┘    └─────────────┘
                          │
                          ▼ (polls every 5s)
                   ┌─────────────┐    ┌─────────────┐
                   │  Ready      │───▶│   Redis     │
                   │  Tasks      │    │   Streams   │
                   └─────────────┘    └─────────────┘
                          │
                          ▼
                   ┌─────────────┐
                   │  Consumer   │
                   │  Processing │
                   └─────────────┘
```

### Delayed Task Features

1. **Redis Sorted Sets Storage**: Tasks stored with execution timestamp as score
2. **Automatic Polling**: Background scheduler polls for ready tasks
3. **Retry Logic**: Failed tasks are automatically retried with exponential backoff
4. **Failed Task Queue**: Tasks exceeding max retries moved to failed queue
5. **Type Safety**: Full protobuf type safety maintained
6. **Statistics**: Real-time monitoring of pending/failed tasks

### Basic Delayed Tasks

```go
// Publish immediately
err := client.Publish(ctx, "user.events", userEvent)

// Publish with delay
err := client.PublishDelayed(ctx, "user.events", userEvent, 5*time.Minute)

// Publish at specific time
scheduleTime := time.Now().Add(2 * time.Hour)
err := client.PublishAt(ctx, "user.events", userEvent, scheduleTime)
```

### Advanced Delayed Task Management

```go
// Start the delayed task scheduler
err := client.StartDelayedScheduler(ctx)

// Get scheduler statistics
stats, err := client.GetDelayedTaskStats(ctx)
fmt.Printf("Pending: %d, Failed: %d\n", stats["pending_tasks"], stats["failed_tasks"])

// Get pending tasks
pending, err := client.GetPendingDelayedTasks(ctx, 10)
for _, task := range pending {
    fmt.Printf("Task %s scheduled for %v\n", task.ID, task.ScheduledAt)
}

// Get failed tasks for manual retry
failed, err := client.GetFailedDelayedTasks(ctx, 10)
```

### Delayed Task Configuration

```go
type DelayedSchedulerConfig struct {
    PollInterval time.Duration // How often to check for ready tasks (default: 5s)
    KeyPrefix    string        // Redis key prefix (default: "delayed_tasks")
    BatchSize    int64         // Max tasks to process per poll (default: 100)
    MaxRetries   int           // Max retry attempts (default: 3)
}
```

### Use Cases for Delayed Tasks

- **Email Notifications**: Send welcome emails 1 hour after signup
- **Reminder Systems**: Payment reminders, appointment notifications
- **Cleanup Jobs**: Delete temporary files after 24 hours
- **Retry Logic**: Retry failed API calls with exponential backoff
- **Scheduled Reports**: Generate daily/weekly reports
- **User Engagement**: Send re-engagement emails after inactivity

## Message Deduplication

The library provides comprehensive message deduplication to prevent duplicate message processing:

### Deduplication Strategies

1. **Content-Based**: Prevents identical messages from being processed multiple times
2. **Idempotency Keys**: Custom keys for business logic deduplication
3. **Business Logic**: Hash based on user actions (user + action = unique)
4. **API Operations**: Prevent duplicate API calls with operation signatures

### How Deduplication Works

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Publisher  │───▶│ Deduplicator│───▶│   Redis     │
│   Publish   │    │ Check Hash  │    │   Storage   │
└─────────────┘    └─────────────┘    └─────────────┘
                          │
                          ▼ (if duplicate)
                   ┌─────────────┐
                   │   Block     │
                   │  Message    │
                   └─────────────┘
                          │
                          ▼ (if unique)
                   ┌─────────────┐
                   │   Redis     │
                   │   Streams   │
                   └─────────────┘
```

### Basic Deduplication Usage

```go
// Content-based deduplication (automatic)
err := client.PublishWithContentDeduplication(ctx, "user.events", userEvent)

// Custom idempotency key
idempotencyKey := "user-signup-12345"
err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, idempotencyKey)

// Business logic deduplication
businessHash := redigo.GenerateBusinessLogicHash("user.events", userID, "account_creation")
err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, businessHash)

// API operation deduplication
apiKey := redigo.GenerateIdempotencyKey("create_account", userEmail, planType)
err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, apiKey)
```

### Advanced Deduplication Management

```go
// Configure deduplication
err := client.SetDeduplicationConfig(redigo.DeduplicationConfig{
    Enabled:   true,
    KeyPrefix: "my_service",
    TTL:       24 * time.Hour, // Remember duplicates for 24 hours
})

// Get deduplication statistics
stats, err := client.GetDeduplicationStats(ctx)
fmt.Printf("Tracked messages: %d\n", stats["total_tracked_messages"])
```

### Deduplication Configuration

```go
type DeduplicationConfig struct {
    Enabled   bool          // Enable/disable deduplication
    KeyPrefix string        // Redis key prefix for deduplication
    TTL       time.Duration // How long to remember message hashes
}
```

### Use Cases

- **API Idempotency**: Prevent duplicate REST API operations
- **User Actions**: Prevent double-clicking, double-submissions
- **Event Sourcing**: Ensure event uniqueness in event streams
- **Payment Processing**: Prevent duplicate payment attempts
- **Notification Systems**: Avoid sending duplicate notifications
- **Data Pipeline**: Prevent duplicate data processing

### Security Features

- **SHA-256 Hashing**: Cryptographically secure message fingerprinting
- **TTL Expiration**: Automatic cleanup of old deduplication records
- **Atomic Operations**: Race-condition-free duplicate checking using Redis SETNX
- **Memory Efficient**: Stores only hash fingerprints, not full messages

## Message Recovery

The library includes a sophisticated message recovery system that handles:

### Automatic Recovery Features

1. **Pending Message Recovery**: Automatically claims and reprocesses messages from failed consumers
2. **Retry Logic**: Configurable retry attempts with intelligent retry pattern detection
3. **Dead Letter Queue**: Messages that exceed retry limits are moved to a dead letter stream
4. **Message Claiming**: Uses Redis Streams XCLAIM to recover orphaned messages
5. **Reprocessing**: Dead letter messages can be manually reprocessed

### How It Works

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Message   │───▶│  Consumer   │───▶│   Handler   │───▶│    Success  │
│   Published │    │   Receives  │    │   Processes │    │     ACK     │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                             │
                                             ▼ (on failure)
                                      ┌─────────────┐
                                      │   Retry     │
                                      │   Logic     │
                                      └─────────────┘
                                             │
                                             ▼ (if retryable)
                                      ┌─────────────┐
                                      │ Re-publish  │
                                      │ with retry  │
                                      │   count     │
                                      └─────────────┘
                                             │
                                             ▼ (max retries exceeded)
                                      ┌─────────────┐
                                      │   Recovery  │
                                      │   System    │
                                      │    XCLAIM   │
                                      └─────────────┘
                                             │
                                             ▼ (still failing)
                                      ┌─────────────┐
                                      │ Dead Letter │
                                      │   Queue     │
                                      └─────────────┘
```

## Why Protocol Buffers?

We chose Protocol Buffers over JSON for message serialization based on several key advantages that are critical for production task queue systems.

### Performance Comparison

Based on industry benchmarks, Protocol Buffers significantly outperform JSON in both speed and memory usage:

| Metric                  | JSON     | Protocol Buffers | Improvement      |
| ----------------------- | -------- | ---------------- | ---------------- |
| **Serialization Speed** | Baseline | 4-10x faster     | 400-1000%        |
| **Message Size**        | Baseline | 2-4x smaller     | 50-75% reduction |
| **Memory Usage**        | Baseline | 2-3x less        | 33-50% reduction |
| **Parsing Speed**       | Baseline | 3-6x faster      | 200-500%         |

_Source: [Rotational Labs Benchmarks](https://rotational.io/blog/go-serialization-formats/)_

### Why This Matters for Task Queues

#### 1. **Network Efficiency**

```go
// JSON message (116 bytes)
{"user_id":"abc123","name":"John Doe","email":"john@example.com","created_at":"2024-01-01T00:00:00Z"}

// Protocol Buffers (28 bytes) - 75% smaller!
// Binary format with field numbers instead of field names
```

#### 2. **Type Safety at Scale**

```go
// JSON approach
type Arg struct {
    Type  string      // "string", "int64", etc.
    Value interface{} // Runtime type assertion required
}

// Our Protocol Buffers approach
message UserCreatedEvent {
    string user_id = 1;    // Compile-time type safety
    string name = 2;       // No runtime type assertions
    string email = 3;      // Schema validation included
}
```

#### 3. **Schema Evolution**

```proto
// Version 1
message UserEvent {
    string user_id = 1;
    string name = 2;
}

// Version 2 - Backward compatible!
message UserEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;        // New field added
    bool is_premium = 4;     // Another new field
}
```

### Comparison with Alternatives

#### vs. JSON

| Aspect               | JSON            | Protocol Buffers  |
| -------------------- | --------------- | ----------------- |
| **Human Readable**   | ✅ Yes          | ❌ Binary format  |
| **Performance**      | ❌ Slow         | ✅ Very fast      |
| **Size**             | ❌ Large        | ✅ Compact        |
| **Type Safety**      | ❌ Runtime only | ✅ Compile-time   |
| **Schema Evolution** | ❌ Manual       | ✅ Automatic      |
| **Cross-Language**   | ⚠️ Limited      | ✅ Full support   |
| **Debugging**        | ✅ Easy         | ⚠️ Requires tools |

#### Example: Traditional vs. Our Approach

**Traditional JSON Approach:**

```go
// Producer
type TaskArgs struct {
    Name string                 `json:"name"`
    Args []interface{}         `json:"args"`
    Headers map[string]interface{} `json:"headers"`
}

taskArgs := &TaskArgs{
    Name: "process_user",
    Args: []interface{}{"user123", "john@example.com"},
    Headers: map[string]interface{}{
        "retry_count": 0,
    },
}

// Consumer - Manual type assertion required
func ProcessUser(args ...interface{}) error {
    userID, ok := args[0].(string)
    if !ok {
        return fmt.Errorf("invalid user_id type")
    }

    email, ok := args[1].(string)
    if !ok {
        return fmt.Errorf("invalid email type")
    }

    // Type safety only at runtime
    // No schema validation
    return processUser(userID, email)
}
```

**Our Approach (Protocol Buffers):**

```go
// Producer
userEvent := &proto.UserCreatedEvent{
    UserId: "user123",
    Email:  "john@example.com",
}
client.Publish(ctx, "user.events", userEvent)

// Consumer - Compile-time type safety
client.Subscribe("user.events", func(ctx context.Context, event *proto.UserCreatedEvent) error {
    // event.UserId and event.Email are guaranteed to be strings
    // Schema is validated at serialization/deserialization
    return processUser(event.UserId, event.Email)
})
```

### When to Choose What?

#### Choose JSON when:

- 🔍 **Debugging is priority** - need to inspect messages easily
- 🏃 **Rapid prototyping** - want to get started quickly
- 📊 **Small scale** - performance isn't critical
- 👥 **Simple types** - basic string/number/bool messages

#### Choose Protocol Buffers (our approach) when:

- ⚡ **Performance matters** - high throughput requirements
- 🏢 **Production systems** - need reliability and efficiency
- 🔒 **Type safety** - want compile-time guarantees
- 📈 **Schema evolution** - need backward compatibility
- 🌐 **Cross-language** - multiple programming languages
- 💾 **Memory efficiency** - resource constraints

### Real-World Impact

For a system processing **100,000 messages/hour**:

| Metric              | JSON         | Protocol Buffers | Savings     |
| ------------------- | ------------ | ---------------- | ----------- |
| **Network Traffic** | 11.6 MB/hour | 2.8 MB/hour      | 8.8 MB/hour |
| **Processing Time** | 540 seconds  | 90 seconds       | 450 seconds |
| **Memory Usage**    | 580 MB       | 193 MB           | 387 MB      |

### Development Experience

#### Type-Safe Message Definitions

```proto
// proto/events.proto
syntax = "proto3";

message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;
    google.protobuf.Timestamp created_at = 4;
}
```

#### Automatic Code Generation

```bash
# Generate Go code from proto definitions
protoc --go_out=. --go_opt=paths=source_relative proto/*.proto
```

#### IntelliSense and Compile-Time Safety

```go
// IDE provides full autocomplete and type checking
userEvent := &proto.UserCreatedEvent{
    UserId: "123",
    Name:   "John",
    // Email: 123,  // Compile error! Must be string
}
```

### Debugging Protocol Buffers

While binary format isn't human-readable, we provide debugging tools:

```go
// Pretty print messages for debugging
fmt.Printf("Message: %s\n", protojson.Format(userEvent))

// Output:
// {
//   "userId": "123",
//   "name": "John",
//   "email": "john@example.com"
// }
```

## Quick Start

### Basic Publisher/Consumer

```go
// Publisher example
publisher := redigo.NewPublisher("redis://localhost:6379")
err := publisher.Publish("user.events", &UserCreatedEvent{
    UserID: "123",
    Email:  "user@example.com",
})

// Consumer example
consumer := redigo.NewConsumer("redis://localhost:6379", "user-service")
consumer.Subscribe("user.events", func(ctx context.Context, msg *UserCreatedEvent) error {
    // Process the message
    return nil
})
consumer.StartConsuming(ctx)
```

### With Message Recovery

```go
// Create consumer
config := redigo.DefaultConsumerConfig("redis://localhost:6379", "my-group", "my-consumer")
consumer, err := redigo.NewConsumerOnly(config)

// Enable recovery with custom settings
consumer.EnableRecovery(redigo.RecoveryConfig{
    IdleTime:         5 * time.Minute,  // Claim messages idle for 5 minutes
    ClaimInterval:    30 * time.Second, // Check every 30 seconds
    MaxRetries:       3,                // Max 3 retries before dead letter
    DeadLetterStream: "my-group:failed", // Custom dead letter stream name
})

// Subscribe with error handling
consumer.Subscribe("events", func(ctx context.Context, event *MyEvent) error {
    // Your processing logic here
    if err := processEvent(event); err != nil {
        // Return retryable errors - they will be retried
        if isTemporaryError(err) {
            return err
        }
        // Non-retryable errors will be acknowledged immediately
        log.Printf("Permanent error: %v", err)
        return nil
    }
    return nil
})
```

### Dead Letter Queue Management

```go
// Get recovery instance
recovery, err := consumer.GetRecovery()

// Check dead letter messages
deadLetters, err := recovery.GetDeadLetterMessages(ctx, 10)
for _, msg := range deadLetters {
    fmt.Printf("Failed message: %s, reason: %s\n",
        msg.ID, msg.Values["reason"])
}

// Reprocess a message from dead letter queue
err = recovery.ReprocessDeadLetterMessage(ctx, deadLetters[0].ID)
```

### Retryable Error Patterns

The system automatically detects retryable errors based on error message content:

```go
// These errors will trigger retries:
- "connection refused"
- "timeout"
- "temporary"
- "network"
- "service unavailable"

// Custom retry logic can be implemented by modifying isRetryableError()
```

## Examples

- [`examples/basic/`](examples/basic/) - Basic publisher/consumer usage
- [`examples/recovery/`](examples/recovery/) - Message recovery demonstration
- [`examples/concurrent/`](examples/concurrent/) - Concurrent processing with worker pools
- [`examples/multi-consumer/`](examples/multi-consumer/) - Multiple consumer safety demonstration
- [`examples/delayed/`](examples/delayed/) - Delayed task scheduling and management
- [`examples/deduplication/`](examples/deduplication/) - Message deduplication strategies and patterns

## Configuration

### Recovery Configuration

```go
type RecoveryConfig struct {
    IdleTime         time.Duration // How long before claiming idle messages
    ClaimInterval    time.Duration // How often to check for claimable messages
    MaxRetries       int           // Maximum retry attempts
    DeadLetterStream string        // Dead letter queue stream name
}
```

### Consumer Configuration

```go
type ConsumerConfig struct {
    RedisURL      string
    ConsumerGroup string
    ConsumerName  string
    BatchSize     int           // Messages to read per batch
    BlockTime     time.Duration // How long to block waiting for messages
    MaxRetries    int           // Default max retries for messages
    RetryBackoff  time.Duration // Backoff between retries
}
```

## Installation

```bash
go get github.com/your-username/strego
```

## Requirements

- Go 1.21+
- Redis 5.0+ (for Redis Streams support)
- Protocol Buffers compiler (for development)

## Development

```bash
# Setup development environment
make dev-setup

# Build the project
make build

# Run tests
make test

# Run recovery example
make redis-start
cd examples/recovery && go run main.go
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License

## Documentation

### Protocol Buffers

- [Protocol Buffers Guide](docs/PROTOBUF_GUIDE.md) - Comprehensive guide to using Protocol Buffers with redigo-streams
- [Serialization Format Comparison](docs/SERIALIZATION_COMPARISON.md) - Detailed comparison of JSON vs Protocol Buffers vs other formats

### Why Protocol Buffers?

- **4-10x faster** serialization than JSON
- **2-4x smaller** message size
- **Compile-time type safety** prevents runtime errors
- **Schema evolution** for backward compatibility
- **Cross-language support** for polyglot microservices

For detailed analysis and benchmarks, see our [Serialization Format Comparison](docs/SERIALIZATION_COMPARISON.md).

## Protocol Buffer Organization

The library uses a clean separation between internal system messages and user-defined messages:

### Internal Proto Messages (`pkg/proto/`)

- **`StreamMessage`** - Internal wrapper for all Redis Stream messages
- **`DelayedTask`** - Internal structure for delayed task scheduling

These are used internally by the library and you typically don't need to interact with them directly.

### Example Proto Messages (`examples/proto/`)

- **`UserCreatedEvent`** - Demo user registration event
- **`EmailSendTask`** - Demo email sending task
- **`OrderCreatedEvent`** - Demo e-commerce order event
- **`PaymentProcessTask`** - Demo payment processing task

These serve as examples and can be copied/modified for your own use cases.

### Creating Your Own Messages

1. **Create your proto file:**

```proto
// myservice/proto/events.proto
syntax = "proto3";

package myservice;
option go_package = "github.com/yourcompany/myservice/proto";

import "google/protobuf/timestamp.proto";

message UserRegistered {
  string user_id = 1;
  string email = 2;
  google.protobuf.Timestamp registered_at = 3;
}
```

2. **Generate Go code:**

```bash
protoc --go_out=. --go_opt=paths=source_relative myservice/proto/events.proto
```

3. **Use in your application:**

```go
import (
    "github.com/yourcompany/myservice/proto"
    "github.com/your-username/strego/pkg/redigo"
)

// Publisher
event := &proto.UserRegistered{
    UserId: "123",
    Email: "user@example.com",
}
err := client.Publish(ctx, "user.events", event)

// Consumer
client.Subscribe("user.events", func(ctx context.Context, event *proto.UserRegistered) error {
    // Process your event
    return nil
})
```

### Proto Best Practices

- **Separate concerns**: Keep internal library messages separate from your business messages
- **Versioning**: Use meaningful package names and plan for schema evolution
- **Documentation**: Add comments to your proto fields for better maintainability
- **Field numbers**: Never reuse field numbers, always add new fields with new numbers
