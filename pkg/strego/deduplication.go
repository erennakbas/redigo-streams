package strego

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// MessageDeduplicator handles message deduplication using Redis
type MessageDeduplicator struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

// DeduplicationConfig configuration for message deduplication
type DeduplicationConfig struct {
	Enabled   bool
	KeyPrefix string
	TTL       time.Duration // How long to remember message IDs
}

// DefaultDeduplicationConfig returns default deduplication configuration
func DefaultDeduplicationConfig() DeduplicationConfig {
	return DeduplicationConfig{
		Enabled:   true,
		KeyPrefix: "dedup",
		TTL:       24 * time.Hour, // Remember messages for 24 hours
	}
}

// NewMessageDeduplicator creates a new message deduplicator
func NewMessageDeduplicator(client *redis.Client, config DeduplicationConfig) *MessageDeduplicator {
	if config.KeyPrefix == "" {
		config.KeyPrefix = "dedup"
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}

	return &MessageDeduplicator{
		client:    client,
		keyPrefix: config.KeyPrefix,
		ttl:       config.TTL,
	}
}

// GenerateMessageHash generates a deterministic hash for a message
func (d *MessageDeduplicator) GenerateMessageHash(stream string, message proto.Message, additionalData ...string) string {
	// Serialize the message to get consistent hash
	data, err := proto.Marshal(message)
	if err != nil {
		// Fallback to message type + additional data
		data = []byte(fmt.Sprintf("%T", message))
	}

	// Create hash from stream + message data + additional context
	hasher := sha256.New()
	hasher.Write([]byte(stream))
	hasher.Write(data)

	// Add additional context data (like user ID, correlation ID, etc.)
	for _, extra := range additionalData {
		hasher.Write([]byte(extra))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// IsDuplicate checks if a message is a duplicate
func (d *MessageDeduplicator) IsDuplicate(ctx context.Context, messageHash string) (bool, error) {
	key := d.getDeduplicationKey(messageHash)

	exists, err := d.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check duplicate: %w", err)
	}

	return exists > 0, nil
}

// MarkAsProcessed marks a message as processed to prevent future duplicates
func (d *MessageDeduplicator) MarkAsProcessed(ctx context.Context, messageHash string, messageID string) error {
	key := d.getDeduplicationKey(messageHash)

	// Store with TTL - value contains message ID and timestamp for debugging
	value := fmt.Sprintf("%s:%d", messageID, time.Now().Unix())

	err := d.client.Set(ctx, key, value, d.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to mark message as processed: %w", err)
	}

	return nil
}

// CheckAndMark atomically checks if message is duplicate and marks it if not
func (d *MessageDeduplicator) CheckAndMark(ctx context.Context, messageHash string, messageID string) (bool, error) {
	key := d.getDeduplicationKey(messageHash)
	value := fmt.Sprintf("%s:%d", messageID, time.Now().Unix())

	// Use Redis SET with NX (only if not exists) and EX (expiration)
	result, err := d.client.SetNX(ctx, key, value, d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check and mark message: %w", err)
	}

	// If result is true, message was not duplicate (we set it successfully)
	// If result is false, message was duplicate (key already existed)
	return !result, nil
}

// GetProcessedInfo gets information about when a message was processed
func (d *MessageDeduplicator) GetProcessedInfo(ctx context.Context, messageHash string) (string, time.Time, error) {
	key := d.getDeduplicationKey(messageHash)

	value, err := d.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", time.Time{}, fmt.Errorf("message not found in deduplication store")
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get processed info: %w", err)
	}

	// Parse value format: "messageID:timestamp"
	var messageID string
	var timestamp int64
	n, err := fmt.Sscanf(value, "%s:%d", &messageID, &timestamp)
	if err != nil || n != 2 {
		return value, time.Time{}, fmt.Errorf("invalid stored value format")
	}

	return messageID, time.Unix(timestamp, 0), nil
}

// CleanupExpired removes expired deduplication entries (optional cleanup)
func (d *MessageDeduplicator) CleanupExpired(ctx context.Context) error {
	// Redis automatically expires keys with TTL, but this method can be used
	// for manual cleanup or statistics
	pattern := d.getDeduplicationKey("*")

	keys, err := d.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get deduplication keys: %w", err)
	}

	var expiredCount int
	for _, key := range keys {
		ttl, err := d.client.TTL(ctx, key).Result()
		if err != nil {
			continue
		}

		if ttl == -2 { // Key doesn't exist (expired)
			expiredCount++
		}
	}

	fmt.Printf("Found %d expired deduplication entries out of %d total\n", expiredCount, len(keys))
	return nil
}

// GetStats returns deduplication statistics
func (d *MessageDeduplicator) GetStats(ctx context.Context) (map[string]interface{}, error) {
	pattern := d.getDeduplicationKey("*")

	keys, err := d.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get deduplication keys: %w", err)
	}

	stats := map[string]interface{}{
		"total_tracked_messages": len(keys),
		"ttl_duration":           d.ttl.String(),
		"key_prefix":             d.keyPrefix,
	}

	return stats, nil
}

// getDeduplicationKey generates the Redis key for deduplication
func (d *MessageDeduplicator) getDeduplicationKey(messageHash string) string {
	return fmt.Sprintf("%s:msg:%s", d.keyPrefix, messageHash)
}

// Content-based deduplication helpers

// GenerateContentHash generates hash based on message content (for identical messages)
func GenerateContentHash(stream string, message proto.Message) string {
	data, _ := proto.Marshal(message)
	hasher := sha256.New()
	hasher.Write([]byte(stream))
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))[:16] // Use first 16 chars for shorter keys
}

// GenerateBusinessLogicHash generates hash based on business logic (e.g., user actions)
func GenerateBusinessLogicHash(stream string, userID string, action string, additionalContext ...string) string {
	hasher := sha256.New()
	hasher.Write([]byte(stream))
	hasher.Write([]byte(userID))
	hasher.Write([]byte(action))

	for _, context := range additionalContext {
		hasher.Write([]byte(context))
	}

	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// IdempotencyKey generates an idempotency key for API operations
func GenerateIdempotencyKey(operation string, params ...string) string {
	hasher := sha256.New()
	hasher.Write([]byte(operation))

	for _, param := range params {
		hasher.Write([]byte(param))
	}

	return hex.EncodeToString(hasher.Sum(nil))[:24] // Longer key for idempotency
}
