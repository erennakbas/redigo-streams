package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	fmt.Println("⏰ [DELAYED CONSUMER] Starting Delayed Task Consumer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create consumer
	config := redigo.DefaultConsumerConfig(redisURL, "delayed-group", "delayed-consumer")
	client, err := redigo.NewConsumerOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	defer client.Close()

	// Subscribe to user events
	err = client.Subscribe("user.events", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("👤 [%s] Processing delayed user event: %s (%s)\n",
			timestamp, event.Name, event.UserId)

		// Check if this was a delayed task or immediate
		isDelayed := isDelayedTask(event.UserId)
		if isDelayed {
			fmt.Printf("   ⏰ This was a delayed task - executed on schedule!\n")
		} else {
			fmt.Printf("   📤 This was an immediate task\n")
		}

		// Simulate user processing
		time.Sleep(200 * time.Millisecond)

		fmt.Printf("   ✅ User %s processed successfully\n\n", event.Name)
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to user events: %v", err)
	}

	// Subscribe to email notifications
	err = client.Subscribe("notifications.email", func(ctx context.Context, task *proto.EmailSendTask) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("📧 [%s] Processing scheduled email: %s\n",
			timestamp, task.Subject)
		fmt.Printf("   📬 Sending to: %s\n", task.To)

		// Simulate email sending
		time.Sleep(300 * time.Millisecond)

		fmt.Printf("   ✅ Scheduled email sent successfully\n\n")
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to email notifications: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✅ Consumer ready! Listening for delayed tasks...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • user.events         - User registration events")
	fmt.Println("   • notifications.email - Email sending tasks")
	fmt.Println("\n📊 Consumer Group: delayed-group")
	fmt.Println("🆔 Consumer ID: delayed-consumer")
	fmt.Println("\n📊 Task processing statistics every 15 seconds")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start consuming
	go func() {
		if err := client.StartConsuming(ctx); err != nil {
			log.Printf("❌ Consumer error: %v", err)
		}
	}()

	// Statistics monitoring
	go monitorConsumerStats(ctx)

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

func isDelayedTask(userID string) bool {
	// Check if the userID indicates this was a delayed task
	delayedPatterns := []string{"delayed-", "batch-"}
	for _, pattern := range delayedPatterns {
		if len(userID) >= len(pattern) && userID[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

func monitorConsumerStats(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	immediateCount := 0
	delayedCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uptime := time.Since(startTime)
			fmt.Printf("📊 [%s] Consumer Stats:\n", time.Now().Format("15:04:05"))
			fmt.Printf("   ⏱️  Uptime: %v\n", uptime.Truncate(time.Second))
			fmt.Printf("   📤 Immediate tasks processed: %d\n", immediateCount)
			fmt.Printf("   ⏰ Delayed tasks processed: %d\n", delayedCount)
			fmt.Printf("   📈 Total processed: %d\n\n", immediateCount+delayedCount)
		}
	}
}
