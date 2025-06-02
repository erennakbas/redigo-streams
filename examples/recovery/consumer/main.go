package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/your-username/redigo-streams/examples/proto"
	"github.com/your-username/redigo-streams/pkg/redigo"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("🔄 [RECOVERY CONSUMER] Starting Message Recovery Demo Consumer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create consumer
	config := redigo.DefaultConsumerConfig(redisURL, "recovery-demo", "recovery-consumer")
	client, err := redigo.NewConsumerOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	defer client.Close()

	// Enable recovery with custom settings
	err = client.EnableRecovery(redigo.RecoveryConfig{
		IdleTime:         30 * time.Second,       // Claim messages idle for 30 seconds
		ClaimInterval:    10 * time.Second,       // Check every 10 seconds
		MaxRetries:       2,                      // Max 2 retries before dead letter
		DeadLetterStream: "recovery-demo:failed", // Dead letter queue
	})
	if err != nil {
		log.Fatalf("❌ Failed to enable recovery: %v", err)
	}
	fmt.Println("✅ Message recovery enabled")

	// Track processing attempts for demo
	processingAttempts := make(map[string]int)

	// Subscribe to recovery test messages with intentional failures
	err = client.Subscribe("test.recovery", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		timestamp := time.Now().Format("15:04:05")

		// Track attempts
		processingAttempts[event.UserId]++
		attempt := processingAttempts[event.UserId]

		fmt.Printf("🔄 [%s] Processing user: %s (attempt #%d)\n",
			timestamp, event.Name, attempt)

		// Simulate different failure scenarios
		if err := simulateProcessingFailures(event, attempt); err != nil {
			fmt.Printf("   ❌ Processing failed: %v\n\n", err)
			return err
		}

		fmt.Printf("   ✅ User %s processed successfully after %d attempts\n\n",
			event.Name, attempt)
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to recovery messages: %v", err)
	}

	// Subscribe to email tasks (these should process normally)
	err = client.Subscribe("email.tasks", func(ctx context.Context, task *proto.EmailSendTask) error {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✅ Consumer ready! Testing message recovery scenarios...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • test.recovery - Recovery test messages")
	fmt.Println("   • email.tasks   - Email sending tasks")
	fmt.Println("\n📊 Consumer Group: recovery-demo")
	fmt.Println("🆔 Consumer ID: recovery-consumer")
	fmt.Println("\n🔄 Recovery Settings:")
	fmt.Println("   • Idle Time: 30 seconds")
	fmt.Println("   • Check Interval: 10 seconds")
	fmt.Println("   • Max Retries: 2")
	fmt.Println("   • Dead Letter: recovery-demo:failed")
	fmt.Println("\n🛑 Press Ctrl+C to stop\n")

	// Start consuming
	go func() {
		if err := client.StartConsuming(ctx); err != nil {
			log.Printf("❌ Consumer error: %v", err)
		}
	}()

	// Monitor recovery process
	go monitorRecoveryProcess(ctx, client)

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down consumer...")
	cancel()
	client.StopConsuming()
	fmt.Println("✅ Consumer stopped cleanly")
}

func simulateProcessingFailures(event *proto.UserCreatedEvent, attempt int) error {
	// Different failure scenarios based on user type and attempt

	if strings.Contains(event.UserId, "problematic") {
		// Problematic users fail multiple times
		if attempt <= 2 {
			return fmt.Errorf("simulated database timeout for problematic user")
		}
		// Success after 2 failures
		time.Sleep(150 * time.Millisecond)
		return nil
	}

	if strings.Contains(event.UserId, "special") {
		// Special users fail once, then succeed
		if attempt == 1 {
			return fmt.Errorf("simulated network error for special user")
		}
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Normal users always succeed
	time.Sleep(50 * time.Millisecond)
	return nil
}

func monitorRecoveryProcess(ctx context.Context, client *redigo.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get recovery instance
			recovery, err := client.GetRecovery()
			if err != nil {
				fmt.Printf("❌ Failed to get recovery instance: %v\n", err)
				continue
			}

			// Check dead letter messages
			deadLetters, err := recovery.GetDeadLetterMessages(ctx, 10)
			if err != nil {
				fmt.Printf("❌ Error getting dead letter messages: %v\n", err)
				continue
			}

			if len(deadLetters) > 0 {
				fmt.Printf("💀 [%s] Found %d messages in dead letter queue:\n",
					time.Now().Format("15:04:05"), len(deadLetters))

				for _, msg := range deadLetters {
					fmt.Printf("   - Message ID: %s\n", msg.ID[:8])
					if originalStream, ok := msg.Values["original_stream"]; ok {
						fmt.Printf("     Original stream: %s\n", originalStream)
					}
					if reason, ok := msg.Values["reason"]; ok {
						fmt.Printf("     Reason: %s\n", reason)
					}
				}

				// Demonstrate reprocessing from dead letter
				if len(deadLetters) > 0 {
					fmt.Printf("\n🔄 Attempting to reprocess first dead letter message...\n")
					if err := recovery.ReprocessDeadLetterMessage(ctx, deadLetters[0].ID); err != nil {
						fmt.Printf("❌ Failed to reprocess: %v\n\n", err)
					} else {
						fmt.Printf("✅ Message queued for reprocessing\n\n")
					}
				}
			} else {
				fmt.Printf("✅ [%s] No messages in dead letter queue\n\n",
					time.Now().Format("15:04:05"))
			}
		}
	}
}
