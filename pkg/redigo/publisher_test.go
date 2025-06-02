package redigo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/your-username/redigo-streams/pkg/proto"
)

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
	userEvent := &pb.UserCreatedEvent{
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
	userEvent := &pb.UserCreatedEvent{
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
	userEvent := &pb.UserCreatedEvent{
		UserId: "test-user-789",
		Email:  "closed@example.com",
		Name:   "Closed User",
	}

	err = publisher.Publish(ctx, "test.closed.events", userEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}
