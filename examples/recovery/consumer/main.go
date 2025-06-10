package main

import (
	"context"
	"fmt"
	"github.com/erennakbas/strego/examples/proto"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/erennakbas/strego"
)

func main() {
	// Get Redis URL from environment or use default
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Create consumer config
	config := strego.DefaultConsumerConfig(redisURL, "recovery-test-group", "recovery-consumer-1")

	// Create consumer
	consumer, err := strego.NewConsumerOnly(config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// Enable recovery with aggressive settings for testing
	recoveryConfig := strego.RecoveryConfig{
		IdleTime:         5 * time.Second, // Claim messages idle for 10 seconds
		ClaimInterval:    2 * time.Second, // Check every 3 seconds
		MaxRetries:       2,               // Max 2 retries before dead letter
		DeadLetterStream: "recovery-test-group:dead-letter",
	}

	err = consumer.EnableRecovery(recoveryConfig)
	if err != nil {
		log.Fatalf("Failed to enable recovery: %v", err)
	}

	// Counter for tracking processing attempts
	var processCount int64

	// Subscribe to events with intentional failures
	err = consumer.Subscribe("test.recovery", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		count := atomic.AddInt64(&processCount, 1)

		fmt.Printf("🎯 Processing attempt #%d for user: %s (ID: %s)\n",
			count, event.GetName(), event.GetUserId())

		// Simulate various failure scenarios
		switch count {
		case 1:
			fmt.Printf("💥 Simulating network timeout failure\n")
			return fmt.Errorf("network timeout")
		case 2:
			fmt.Printf("💥 Simulating database connection failure\n")
			return fmt.Errorf("database connection failed")
		case 3:
			fmt.Printf("✅ Third time's the charm! Processing successfully\n")
			return nil // Success
		default:
			fmt.Printf("✅ Processing successful\n")
			return nil
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	// Subscribe to email tasks (these should process normally)
	err = consumer.Subscribe("email.tasks", func(ctx context.Context, task *proto.EmailSendTask) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("📧 [%s] Processing email task: %s\n",
			timestamp, task.Subject)

		// Simulate email processing
		time.Sleep(100 * time.Millisecond)

		fmt.Printf("   ✅ Email sent successfully to %s\n\n", task.To)
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to email tasks: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consuming
	fmt.Printf("🚀 Starting recovery consumer...\n")
	fmt.Printf("📝 Recovery Config: IdleTime=%v, ClaimInterval=%v, MaxRetries=%d\n",
		recoveryConfig.IdleTime, recoveryConfig.ClaimInterval, recoveryConfig.MaxRetries)

	err = consumer.StartConsuming(ctx)
	if err != nil {
		log.Fatalf("Failed to start consuming: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("✅ Consumer started!")
	fmt.Printf("💡 Press Ctrl+C to stop\n")

	<-sigChan
	fmt.Printf("\n🛑 Shutting down consumer...\n")

	cancel()

	fmt.Printf("👋 Consumer stopped gracefully\n")
}
