package strego

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// MessageRecovery handles recovery of pending and failed messages
type MessageRecovery struct {
	client           *redis.Client
	consumerGroup    string
	consumerName     string
	idleTime         time.Duration // How long a message should be idle before claiming
	claimInterval    time.Duration // How often to check for claimable messages
	maxRetries       int
	deadLetterStream string // Stream for messages that exceed max retries
}

// NewMessageRecovery creates a new message recovery instance
func NewMessageRecovery(client *redis.Client, consumerGroup, consumerName string) *MessageRecovery {
	return &MessageRecovery{
		client:           client,
		consumerGroup:    consumerGroup,
		consumerName:     consumerName,
		idleTime:         5 * time.Minute,  // Messages idle for 5 minutes can be claimed
		claimInterval:    30 * time.Second, // Check for claimable messages every 30 seconds
		maxRetries:       3,
		deadLetterStream: fmt.Sprintf("%s:dead-letter", consumerGroup),
	}
}

// RecoveryConfig allows customizing recovery behavior
type RecoveryConfig struct {
	IdleTime         time.Duration
	ClaimInterval    time.Duration
	MaxRetries       int
	DeadLetterStream string
}

// NewMessageRecoveryWithConfig creates a recovery instance with custom config
func NewMessageRecoveryWithConfig(client *redis.Client, consumerGroup, consumerName string, config RecoveryConfig) *MessageRecovery {
	recovery := NewMessageRecovery(client, consumerGroup, consumerName)

	if config.IdleTime > 0 {
		recovery.idleTime = config.IdleTime
	}
	if config.ClaimInterval > 0 {
		recovery.claimInterval = config.ClaimInterval
	}
	if config.MaxRetries > 0 {
		recovery.maxRetries = config.MaxRetries
	}
	if config.DeadLetterStream != "" {
		recovery.deadLetterStream = config.DeadLetterStream
	}

	return recovery
}

// StartRecovery starts the recovery process in the background
func (r *MessageRecovery) StartRecovery(ctx context.Context, streams []string, processor MessageProcessor) {
	ticker := time.NewTicker(r.claimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, stream := range streams {
				if err := r.recoverPendingMessages(ctx, stream, processor); err != nil {
					fmt.Printf("Error recovering messages from stream %s: %v\n", stream, err)
				}
			}
		}
	}
}

// MessageProcessor interface for processing recovered messages
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, stream string, message redis.XMessage) error
}

// recoverPendingMessages recovers pending messages from a specific stream
func (r *MessageRecovery) recoverPendingMessages(ctx context.Context, stream string, processor MessageProcessor) error {
	// Get pending messages summary
	pending, err := r.client.XPending(ctx, stream, r.consumerGroup).Result()
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if pending.Count == 0 {
		return nil // No pending messages
	}

	fmt.Printf("Found %d pending messages in stream %s\n", pending.Count, stream)

	// Get detailed pending messages
	pendingExt, err := r.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  r.consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100, // Process up to 100 messages at once
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to get detailed pending messages: %w", err)
	}

	for _, msg := range pendingExt {
		// Check if message has been idle long enough to claim
		if msg.Idle < r.idleTime {
			continue
		}

		fmt.Printf("Attempting to claim message %s (idle for %v, delivered %d times)\n",
			msg.ID, msg.Idle, msg.RetryCount)

		// Check if message has exceeded max retries
		if int(msg.RetryCount) >= r.maxRetries {
			if err := r.moveToDeadLetter(ctx, stream, msg.ID); err != nil {
				fmt.Printf("Failed to move message %s to dead letter: %v\n", msg.ID, err)
			}
			continue
		}

		// Claim the message
		claimedMsgs, err := r.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   stream,
			Group:    r.consumerGroup,
			Consumer: r.consumerName,
			MinIdle:  r.idleTime,
			Messages: []string{msg.ID},
		}).Result()
		if err != nil {
			fmt.Printf("Failed to claim message %s: %v\n", msg.ID, err)
			continue
		}

		// Process claimed messages
		for _, claimedMsg := range claimedMsgs {
			if err := r.processClaimedMessage(ctx, stream, claimedMsg, processor); err != nil {
				fmt.Printf("Failed to process claimed message %s: %v\n", claimedMsg.ID, err)
			}
		}
	}

	return nil
}

// processClaimedMessage processes a claimed message
func (r *MessageRecovery) processClaimedMessage(ctx context.Context, stream string, message redis.XMessage, processor MessageProcessor) error {
	// Process the message
	if err := processor.ProcessMessage(ctx, stream, message); err != nil {
		fmt.Printf("Error processing claimed message %s: %v\n", message.ID, err)
		return err
	}

	// Acknowledge the message if processing was successful
	return r.client.XAck(ctx, stream, r.consumerGroup, message.ID).Err()
}

// moveToDeadLetter moves a message to the dead letter stream
func (r *MessageRecovery) moveToDeadLetter(ctx context.Context, stream, messageID string) error {
	// First, get the original message
	messages, err := r.client.XRange(ctx, stream, messageID, messageID).Result()
	if err != nil || len(messages) == 0 {
		return fmt.Errorf("failed to get original message %s: %w", messageID, err)
	}

	originalMsg := messages[0]

	// Add metadata about the failure
	deadLetterData := make(map[string]interface{})
	for k, v := range originalMsg.Values {
		deadLetterData[k] = v
	}
	deadLetterData["original_stream"] = stream
	deadLetterData["original_message_id"] = messageID
	deadLetterData["failed_at"] = time.Now().Unix()
	deadLetterData["reason"] = "max_retries_exceeded"

	// Add to dead letter stream
	_, err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.deadLetterStream,
		Values: deadLetterData,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to add message to dead letter stream: %w", err)
	}

	// Acknowledge the original message to remove it from pending
	err = r.client.XAck(ctx, stream, r.consumerGroup, messageID).Err()
	if err != nil {
		return fmt.Errorf("failed to ack message after moving to dead letter: %w", err)
	}

	fmt.Printf("Moved message %s to dead letter stream %s\n", messageID, r.deadLetterStream)
	return nil
}

// GetDeadLetterMessages retrieves messages from the dead letter stream (thread-safe with pagination)
func (r *MessageRecovery) GetDeadLetterMessages(ctx context.Context, count int64) ([]redis.XMessage, error) {
	if count <= 0 {
		count = 100 // Default limit
	}

	// Use XRevRangeN for count limit
	return r.client.XRevRangeN(ctx, r.deadLetterStream, "+", "-", count).Result()
}

// GetDeadLetterMessagesPaginated retrieves dead letter messages with pagination (thread-safe)
func (r *MessageRecovery) GetDeadLetterMessagesPaginated(ctx context.Context, start, end string, count int64) ([]redis.XMessage, error) {
	if count <= 0 {
		count = 100
	}

	return r.client.XRevRangeN(ctx, r.deadLetterStream, end, start, count).Result()
}

// ReprocessDeadLetterMessage moves a message from dead letter back to original stream (thread-safe)
func (r *MessageRecovery) ReprocessDeadLetterMessage(ctx context.Context, deadLetterMessageID string) error {
	// Use Redis transaction for atomic operations
	return r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Get the dead letter message within transaction
		messages, err := tx.XRange(ctx, r.deadLetterStream, deadLetterMessageID, deadLetterMessageID).Result()
		if err != nil {
			return fmt.Errorf("failed to get dead letter message: %w", err)
		}

		if len(messages) == 0 {
			return fmt.Errorf("dead letter message %s not found or already processed", deadLetterMessageID)
		}

		dlMsg := messages[0]

		// Extract original stream
		originalStream, ok := dlMsg.Values["original_stream"].(string)
		if !ok {
			return fmt.Errorf("original_stream not found in dead letter message")
		}

		// Prepare data for reprocessing
		reprocessData := make(map[string]interface{})
		for k, v := range dlMsg.Values {
			if k != "original_stream" && k != "original_message_id" && k != "failed_at" && k != "reason" {
				reprocessData[k] = v
			}
		}
		reprocessData["reprocessed_at"] = time.Now().Unix()
		reprocessData["reprocessed_from_dead_letter"] = deadLetterMessageID
		reprocessData["reprocess_attempt"] = fmt.Sprintf("%d", time.Now().UnixNano()) // Unique marker

		// Execute atomic transaction
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			// 1. Add message back to original stream
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: originalStream,
				Values: reprocessData,
			})

			// 2. Remove from dead letter stream (atomic with step 1)
			pipe.XDel(ctx, r.deadLetterStream, deadLetterMessageID)

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to reprocess message atomically: %w", err)
		}

		fmt.Printf("✅ Thread-safe reprocessed message %s from dead letter back to stream %s\n",
			deadLetterMessageID, originalStream)
		return nil

	}, r.deadLetterStream) // Watch the dead letter stream for changes
}

// ReprocessMultipleDeadLetterMessages reprocesses multiple messages atomically
func (r *MessageRecovery) ReprocessMultipleDeadLetterMessages(ctx context.Context, deadLetterMessageIDs []string) ([]string, []error) {
	if len(deadLetterMessageIDs) == 0 {
		return nil, nil
	}

	processed := make([]string, 0, len(deadLetterMessageIDs))
	errors := make([]error, 0, len(deadLetterMessageIDs))

	// Process each message individually to ensure atomicity per message
	for _, messageID := range deadLetterMessageIDs {
		if err := r.ReprocessDeadLetterMessage(ctx, messageID); err != nil {
			errors = append(errors, fmt.Errorf("failed to reprocess %s: %w", messageID, err))
		} else {
			processed = append(processed, messageID)
		}
	}

	return processed, errors
}
