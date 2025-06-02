package strego

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test message that properly implements proto.Message interface
type TestUserEvent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserId    string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Email     string                 `protobuf:"bytes,2,opt,name=email,proto3" json:"email,omitempty"`
	Name      string                 `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	CreatedAt *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
}

func (x *TestUserEvent) Reset() {
	*x = TestUserEvent{}
	if protoimpl.UnsafeEnabled {
		mi := &testUserEventMsgInfo
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestUserEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestUserEvent) ProtoMessage() {}

func (x *TestUserEvent) ProtoReflect() protoreflect.Message {
	mi := &testUserEventMsgInfo
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var testUserEventMsgInfo = protoimpl.MessageInfo{
	GoReflectType: reflect.TypeOf((*TestUserEvent)(nil)),
}

func TestNewPublisher(t *testing.T) {
	config := PublisherConfig{
		RedisURL:     "redis://localhost:6379",
		MaxRetries:   3,
		RetryBackoff: time.Second,
	}

	publisher, err := NewPublisher(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	require.NoError(t, err)
	require.NotNil(t, publisher)

	defer publisher.Close()
}

func TestPublisher_Publish(t *testing.T) {
	config := PublisherConfig{
		RedisURL:     "redis://localhost:6379",
		MaxRetries:   3,
		RetryBackoff: time.Second,
	}

	publisher, err := NewPublisher(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	require.NoError(t, err)
	defer publisher.Close()

	ctx := context.Background()

	// Test publishing a user created event
	userEvent := &TestUserEvent{
		UserId: "test-user-123",
		Email:  "test@example.com",
		Name:   "Test User",
	}

	err = publisher.Publish(ctx, "test.user.events", userEvent)
	assert.NoError(t, err)
}

func TestPublisher_PublishDelayed(t *testing.T) {
	config := PublisherConfig{
		RedisURL:     "redis://localhost:6379",
		MaxRetries:   3,
		RetryBackoff: time.Second,
	}

	publisher, err := NewPublisher(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	require.NoError(t, err)
	defer publisher.Close()

	ctx := context.Background()

	// Test delayed publishing (currently returns error as not implemented)
	userEvent := &TestUserEvent{
		UserId: "test-user-456",
		Email:  "delayed@example.com",
		Name:   "Delayed User",
	}

	err = publisher.PublishDelayed(ctx, "test.delayed.events", userEvent, 5*time.Second)
	assert.Error(t, err) // Should error as delayed publishing is not yet implemented
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestPublisher_Close(t *testing.T) {
	config := PublisherConfig{
		RedisURL:     "redis://localhost:6379",
		MaxRetries:   3,
		RetryBackoff: time.Second,
	}

	publisher, err := NewPublisher(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	require.NoError(t, err)

	err = publisher.Close()
	assert.NoError(t, err)

	// Test publishing after close should error
	ctx := context.Background()
	userEvent := &TestUserEvent{
		UserId: "test-user-789",
		Email:  "closed@example.com",
		Name:   "Closed User",
	}

	err = publisher.Publish(ctx, "test.closed.events", userEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}
