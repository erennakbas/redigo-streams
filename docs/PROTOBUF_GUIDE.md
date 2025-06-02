# Protocol Buffers Guide

This guide covers everything you need to know about using Protocol Buffers with redigo-streams.

## Table of Contents

- [Getting Started](#getting-started)
- [Message Definition](#message-definition)
- [Code Generation](#code-generation)
- [Schema Evolution](#schema-evolution)
- [Best Practices](#best-practices)
- [Performance Tips](#performance-tips)
- [Debugging](#debugging)
- [Common Patterns](#common-patterns)
- [Troubleshooting](#troubleshooting)

## Getting Started

### Prerequisites

Install the Protocol Buffers compiler:

```bash
# macOS
brew install protobuf

# Ubuntu
sudo apt install protobuf-compiler

# Or download from: https://github.com/protocolbuffers/protobuf/releases
```

Install Go protobuf plugins:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Project Structure

```
your-project/
├── proto/
│   ├── events.proto
│   ├── tasks.proto
│   └── common.proto
├── pkg/
│   └── proto/
│       ├── events.pb.go
│       ├── tasks.pb.go
│       └── common.pb.go
└── go.mod
```

## Message Definition

### Basic Message Types

```proto
// proto/events.proto
syntax = "proto3";

package events.v1;
option go_package = "github.com/your-username/your-project/pkg/proto";

import "google/protobuf/timestamp.proto";

// User lifecycle events
message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;
    google.protobuf.Timestamp created_at = 4;
    UserProfile profile = 5;
}

message UserUpdatedEvent {
    string user_id = 1;
    repeated string updated_fields = 2;
    UserProfile old_profile = 3;
    UserProfile new_profile = 4;
    google.protobuf.Timestamp updated_at = 5;
}

message UserDeletedEvent {
    string user_id = 1;
    string reason = 2;
    google.protobuf.Timestamp deleted_at = 3;
}
```

### Nested Messages

```proto
message UserProfile {
    string first_name = 1;
    string last_name = 2;
    int32 age = 3;
    Address address = 4;
    repeated string interests = 5;
}

message Address {
    string street = 1;
    string city = 2;
    string country = 3;
    string postal_code = 4;
}
```

### Enums

```proto
enum UserStatus {
    USER_STATUS_UNSPECIFIED = 0;
    USER_STATUS_ACTIVE = 1;
    USER_STATUS_INACTIVE = 2;
    USER_STATUS_SUSPENDED = 3;
    USER_STATUS_DELETED = 4;
}

message UserStatusChangedEvent {
    string user_id = 1;
    UserStatus old_status = 2;
    UserStatus new_status = 3;
    string reason = 4;
    google.protobuf.Timestamp changed_at = 5;
}
```

### Task Messages

```proto
// proto/tasks.proto
syntax = "proto3";

package tasks.v1;
option go_package = "github.com/your-username/your-project/pkg/proto";

import "google/protobuf/timestamp.proto";

message EmailSendTask {
    string task_id = 1;
    string to = 2;
    string from = 3;
    string subject = 4;
    string body = 5;
    EmailType type = 6;
    map<string, string> headers = 7;
    google.protobuf.Timestamp scheduled_at = 8;
}

enum EmailType {
    EMAIL_TYPE_UNSPECIFIED = 0;
    EMAIL_TYPE_WELCOME = 1;
    EMAIL_TYPE_NOTIFICATION = 2;
    EMAIL_TYPE_MARKETING = 3;
    EMAIL_TYPE_TRANSACTIONAL = 4;
}

message NotificationTask {
    string user_id = 1;
    string title = 2;
    string message = 3;
    NotificationChannel channel = 4;
    map<string, string> metadata = 5;
}

enum NotificationChannel {
    NOTIFICATION_CHANNEL_UNSPECIFIED = 0;
    NOTIFICATION_CHANNEL_EMAIL = 1;
    NOTIFICATION_CHANNEL_SMS = 2;
    NOTIFICATION_CHANNEL_PUSH = 3;
    NOTIFICATION_CHANNEL_IN_APP = 4;
}
```

## Code Generation

### Makefile Setup

```makefile
# Makefile
PROTO_DIR := proto
PROTO_OUT_DIR := pkg/proto

.PHONY: proto-gen
proto-gen:
	@echo "Generating Protocol Buffers..."
	@mkdir -p $(PROTO_OUT_DIR)
	@protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_OUT_DIR) \
		--go_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto
	@echo "Protocol Buffers generated successfully"

.PHONY: proto-clean
proto-clean:
	@echo "Cleaning generated Protocol Buffers..."
	@rm -rf $(PROTO_OUT_DIR)/*.pb.go

.PHONY: proto-validate
proto-validate:
	@echo "Validating Protocol Buffers..."
	@protoc \
		--proto_path=$(PROTO_DIR) \
		--descriptor_set_out=/dev/null \
		$(PROTO_DIR)/*.proto
	@echo "All proto files are valid"
```

### Build Script

```bash
#!/bin/bash
# scripts/generate-proto.sh

set -e

PROTO_DIR="proto"
OUTPUT_DIR="pkg/proto"

echo "🔧 Generating Protocol Buffers..."

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Generate Go code
protoc \
    --proto_path="$PROTO_DIR" \
    --go_out="$OUTPUT_DIR" \
    --go_opt=paths=source_relative \
    "$PROTO_DIR"/*.proto

echo "✅ Protocol Buffers generated successfully"

# Optional: Format generated code
go fmt "$OUTPUT_DIR"/*.go

echo "📦 Generated files:"
ls -la "$OUTPUT_DIR"/*.pb.go
```

## Schema Evolution

### Adding Fields (Backward Compatible)

```proto
// Version 1
message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;
}

// Version 2 - Safe to add new fields
message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;
    google.protobuf.Timestamp created_at = 4;  // New field
    UserProfile profile = 5;                   // New field
    bool is_verified = 6;                      // New field
}
```

### Field Deprecation

```proto
message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;

    // Deprecated fields - keep for backward compatibility
    string old_field = 4 [deprecated = true];

    // New fields
    google.protobuf.Timestamp created_at = 5;
    UserProfile profile = 6;
}
```

### Renaming Fields (Safe)

```proto
// Field names can be changed without breaking compatibility
message UserCreatedEvent {
    string user_id = 1;
    string full_name = 2;      // Was "name" - field number stays same
    string email_address = 3;  // Was "email" - field number stays same
}
```

### Reserved Fields

```proto
message UserCreatedEvent {
    // Reserve field numbers and names that were removed
    reserved 10 to 15;
    reserved "old_deprecated_field", "another_old_field";

    string user_id = 1;
    string name = 2;
    string email = 3;
}
```

## Best Practices

### Field Numbering

```proto
message UserEvent {
    // Use 1-15 for frequently used fields (1 byte encoding)
    string user_id = 1;
    string action = 2;
    google.protobuf.Timestamp timestamp = 3;

    // Use 16+ for less frequent fields (2+ byte encoding)
    map<string, string> metadata = 16;
    repeated string tags = 17;
}
```

### Naming Conventions

```proto
// Use UPPER_SNAKE_CASE for enums
enum OrderStatus {
    ORDER_STATUS_UNSPECIFIED = 0;
    ORDER_STATUS_PENDING = 1;
    ORDER_STATUS_CONFIRMED = 2;
    ORDER_STATUS_SHIPPED = 3;
    ORDER_STATUS_DELIVERED = 4;
}

// Use snake_case for fields
message Order {
    string order_id = 1;
    OrderStatus order_status = 2;
    google.protobuf.Timestamp created_at = 3;
    repeated OrderItem order_items = 4;
}
```

### Optional vs Required

```proto
// Proto3 style - all fields are optional by default
message UserEvent {
    string user_id = 1;           // Required in business logic
    string name = 2;              // Optional
    string email = 3;             // Optional

    // Use optional keyword for explicit optionality
    optional string phone = 4;
}
```

### Common Types

```proto
// proto/common.proto
syntax = "proto3";

package common.v1;
option go_package = "github.com/your-username/your-project/pkg/proto";

import "google/protobuf/timestamp.proto";

// Standard metadata for all events
message EventMetadata {
    string event_id = 1;
    string correlation_id = 2;
    string source_service = 3;
    google.protobuf.Timestamp created_at = 4;
    map<string, string> headers = 5;
}

// Standard pagination
message Pagination {
    int32 page = 1;
    int32 page_size = 2;
    int32 total = 3;
}

// Standard error information
message ErrorInfo {
    string code = 1;
    string message = 2;
    repeated string details = 3;
}
```

## Performance Tips

### Efficient Field Access

```go
// Efficient - direct field access
func processUser(event *proto.UserCreatedEvent) {
    userID := event.GetUserId()    // Fast
    name := event.GetName()        // Fast
    email := event.GetEmail()      // Fast
}

// Inefficient - reflection or JSON conversion
func processUserSlow(event *proto.UserCreatedEvent) {
    // Don't do this in hot paths
    jsonBytes, _ := protojson.Marshal(event)
    var data map[string]interface{}
    json.Unmarshal(jsonBytes, &data)
}
```

### Memory Optimization

```go
// Reuse message objects when possible
var userEventPool = sync.Pool{
    New: func() interface{} {
        return &proto.UserCreatedEvent{}
    },
}

func processMessage(data []byte) error {
    event := userEventPool.Get().(*proto.UserCreatedEvent)
    defer func() {
        event.Reset() // Clear the message
        userEventPool.Put(event)
    }()

    if err := proto.Unmarshal(data, event); err != nil {
        return err
    }

    // Process event...
    return nil
}
```

### Batch Processing

```go
// Process multiple messages efficiently
func processBatch(events []*proto.UserCreatedEvent) error {
    // Pre-allocate slices
    userIDs := make([]string, 0, len(events))
    emails := make([]string, 0, len(events))

    for _, event := range events {
        userIDs = append(userIDs, event.GetUserId())
        emails = append(emails, event.GetEmail())
    }

    // Batch database operations
    return batchCreateUsers(userIDs, emails)
}
```

## Debugging

### Pretty Printing

```go
import (
    "fmt"
    "google.golang.org/protobuf/encoding/protojson"
)

func debugMessage(event *proto.UserCreatedEvent) {
    // Pretty print as JSON
    jsonBytes, err := protojson.MarshalOptions{
        Multiline:       true,
        Indent:          "  ",
        EmitUnpopulated: true,
    }.Marshal(event)

    if err != nil {
        fmt.Printf("Error marshaling: %v\n", err)
        return
    }

    fmt.Printf("Event: %s\n", string(jsonBytes))
}
```

### Message Validation

```go
import "google.golang.org/protobuf/proto"

func validateMessage(event *proto.UserCreatedEvent) error {
    // Check required fields
    if event.GetUserId() == "" {
        return fmt.Errorf("user_id is required")
    }

    if event.GetEmail() == "" {
        return fmt.Errorf("email is required")
    }

    // Validate email format
    if !isValidEmail(event.GetEmail()) {
        return fmt.Errorf("invalid email format")
    }

    return nil
}
```

### Binary Inspection

```bash
# Install protobuf tools
go install github.com/protocolbuffers/protobuf-go/cmd/protoc-gen-go@latest

# Decode binary protobuf (if you have the .proto file)
protoc --decode=events.v1.UserCreatedEvent \
       --proto_path=proto \
       proto/events.proto < message.bin
```

## Common Patterns

### Event Sourcing

```proto
message EventEnvelope {
    string event_id = 1;
    string stream_id = 2;
    int64 version = 3;
    string event_type = 4;
    google.protobuf.Any event_data = 5;
    google.protobuf.Timestamp created_at = 6;
    map<string, string> metadata = 7;
}
```

### Command Query Separation

```proto
// Commands (requests to change state)
message CreateUserCommand {
    string user_id = 1;
    string name = 2;
    string email = 3;
}

// Events (notifications of state changes)
message UserCreatedEvent {
    string user_id = 1;
    string name = 2;
    string email = 3;
    google.protobuf.Timestamp created_at = 4;
}

// Queries (requests for data)
message GetUserQuery {
    string user_id = 1;
}

message GetUserResponse {
    UserProfile user = 1;
    bool found = 2;
}
```

### Polymorphic Messages

```proto
import "google/protobuf/any.proto";

message TaskWrapper {
    string task_id = 1;
    string task_type = 2;
    google.protobuf.Any task_data = 3;
    google.protobuf.Timestamp scheduled_at = 4;
}

// Usage
func createEmailTask() *proto.TaskWrapper {
    emailTask := &proto.EmailSendTask{
        To:      "user@example.com",
        Subject: "Welcome!",
        Body:    "Welcome to our service",
    }

    taskData, _ := anypb.New(emailTask)

    return &proto.TaskWrapper{
        TaskId:      generateID(),
        TaskType:    "email_send",
        TaskData:    taskData,
        ScheduledAt: timestamppb.Now(),
    }
}
```

## Troubleshooting

### Common Errors

#### 1. Import Path Issues

```go
// Error: cannot find package
import "github.com/your-username/your-project/pkg/proto"

// Solution: Check go.mod and import paths
module github.com/your-username/your-project

// In proto files:
option go_package = "github.com/your-username/your-project/pkg/proto";
```

#### 2. Field Number Conflicts

```proto
// Error: field number conflicts
message UserEvent {
    string user_id = 1;
    string name = 1;     // Error: duplicate field number
}

// Solution: Use unique field numbers
message UserEvent {
    string user_id = 1;
    string name = 2;     // Unique field number
}
```

#### 3. Type Assertion Errors

```go
// Error: interface conversion panic
func processAny(anyMsg *anypb.Any) {
    userEvent := &proto.UserCreatedEvent{}
    anyMsg.UnmarshalTo(userEvent)  // May fail
}

// Solution: Check type before unmarshaling
func processAny(anyMsg *anypb.Any) error {
    if anyMsg.MessageIs(&proto.UserCreatedEvent{}) {
        userEvent := &proto.UserCreatedEvent{}
        return anyMsg.UnmarshalTo(userEvent)
    }
    return fmt.Errorf("unexpected message type: %s", anyMsg.TypeUrl)
}
```

### Performance Issues

#### 1. Large Message Size

```proto
// Problem: Very large messages
message LargeEvent {
    repeated string huge_list = 1;  // Millions of items
}

// Solution: Use streaming or pagination
message EventBatch {
    repeated Event events = 1;
    string next_token = 2;
    bool has_more = 3;
}
```

#### 2. Excessive Memory Usage

```go
// Problem: Not reusing message objects
func processMessages(data [][]byte) {
    for _, msgData := range data {
        event := &proto.UserCreatedEvent{}  // New allocation each time
        proto.Unmarshal(msgData, event)
        // process...
    }
}

// Solution: Use object pooling (see Performance Tips)
```

### Development Workflow

```bash
# 1. Make proto changes
vim proto/events.proto

# 2. Validate proto files
make proto-validate

# 3. Generate Go code
make proto-gen

# 4. Run tests
go test ./...

# 5. Build project
go build ./...
```

## References

- [Protocol Buffers Language Guide](https://developers.google.com/protocol-buffers/docs/proto3)
- [Go Generated Code Guide](https://developers.google.com/protocol-buffers/docs/reference/go-generated)
- [Protocol Buffers Best Practices](https://developers.google.com/protocol-buffers/docs/style)
- [Performance Tips](https://developers.google.com/protocol-buffers/docs/techniques)
