package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erennakbas/strego/examples/proto"
	"github.com/erennakbas/strego/pkg/strego"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("👥 [MULTI-CONSUMER PRODUCER] Starting Multi-Consumer Safety Test Producer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create publisher
	config := strego.DefaultPublisherConfig(redisURL)
	client, err := strego.NewPublisherOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create publisher: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\n👥 Publishing messages for multi-consumer safety testing...")
	fmt.Println("📋 Will publish:")
	fmt.Println("   • Sequential user events")
	fmt.Println("   • Mixed email tasks")
	fmt.Println("   • Burst messages for load testing")
	fmt.Println("\n💡 Multiple consumers should each process different messages")
	fmt.Println("🔒 No message should be processed by multiple consumers")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start publishing multi-consumer test messages
	go publishMultiConsumerTestMessages(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	fmt.Println("✅ Producer stopped cleanly")
}

func publishMultiConsumerTestMessages(ctx context.Context, client *strego.Client) {
	// Give consumers time to start
	time.Sleep(2 * time.Second)

	batchCounter := 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("\n📤 Publishing batch %d...\n", batchCounter)

			// 1. Sequential user events
			publishSequentialUsers(ctx, client, batchCounter)
			time.Sleep(2 * time.Second)

			// 2. Mixed email tasks
			publishMixedEmails(ctx, client, batchCounter)
			time.Sleep(2 * time.Second)

			// 3. Burst messages to test consumer distribution
			publishBurstMessages(ctx, client, batchCounter)

			batchCounter++

			// Wait before next batch
			fmt.Printf("\n⏱️  Waiting 15 seconds before next batch...\n")
			time.Sleep(15 * time.Second)
		}
	}
}

func publishSequentialUsers(ctx context.Context, client *strego.Client, batch int) {
	fmt.Printf("👤 Publishing sequential users for batch %d...\n", batch)

	// Publish 12 user events sequentially
	for i := 1; i <= 12; i++ {
		userEvent := &proto.UserCreatedEvent{
			UserId: fmt.Sprintf("batch%d-seq-%03d", batch, i), // Sequential ID for tracking
			Email:  fmt.Sprintf("seq%d-%d@example.com", batch, i),
			Name:   fmt.Sprintf("Sequential User %d-%d", batch, i),
		}

		err := client.Publish(ctx, "test.safety", userEvent)
		if err != nil {
			log.Printf("❌ Failed to publish sequential user %d: %v", i, err)
		} else {
			fmt.Printf("📤 [%s] Published sequential user: %s\n",
				time.Now().Format("15:04:05"), userEvent.UserId)
		}

		// Small delay to see distribution
		time.Sleep(200 * time.Millisecond)
	}
}

func publishMixedEmails(ctx context.Context, client *strego.Client, batch int) {
	fmt.Printf("📧 Publishing mixed email tasks for batch %d...\n", batch)

	emailTypes := []string{"welcome", "verification", "reset", "newsletter", "notification"}

	// Publish 8 email tasks
	for i := 1; i <= 8; i++ {
		emailType := emailTypes[(i-1)%len(emailTypes)]

		emailTask := &proto.EmailSendTask{
			To:      fmt.Sprintf("batch%d-email%d@example.com", batch, i),
			Subject: fmt.Sprintf("[Batch %d] %s Email #%d", batch, emailType, i),
			Body:    fmt.Sprintf("This is a %s email for batch %d, task %d", emailType, batch, i),
			Variables: map[string]string{
				"batch":      fmt.Sprintf("%d", batch),
				"email_type": emailType,
				"task_id":    fmt.Sprintf("%d", i),
			},
		}

		err := client.Publish(ctx, "email.tasks", emailTask)
		if err != nil {
			log.Printf("❌ Failed to publish email task %d: %v", i, err)
		} else {
			fmt.Printf("📧 [%s] Published email task: %s\n",
				time.Now().Format("15:04:05"), emailTask.Subject)
		}

		time.Sleep(250 * time.Millisecond)
	}
}

func publishBurstMessages(ctx context.Context, client *strego.Client, batch int) {
	fmt.Printf("⚡ Publishing burst messages for batch %d...\n", batch)

	// Publish 20 messages in quick succession to test load balancing
	for i := 1; i <= 20; i++ {
		// Alternate between user events and special safety test messages
		if i%3 == 0 {
			// Special safety test user
			userEvent := &proto.UserCreatedEvent{
				UserId: fmt.Sprintf("batch%d-safety-%03d", batch, i),
				Email:  fmt.Sprintf("safety%d-%d@example.com", batch, i),
				Name:   fmt.Sprintf("Safety Test User %d-%d", batch, i),
			}

			err := client.Publish(ctx, "test.safety", userEvent)
			if err != nil {
				log.Printf("❌ Failed to publish safety user: %v", err)
			} else {
				fmt.Printf("🔒 [%s] Safety test user: %s\n",
					time.Now().Format("15:04:05"), userEvent.UserId)
			}
		} else {
			// Regular burst user
			userEvent := &proto.UserCreatedEvent{
				UserId: fmt.Sprintf("batch%d-burst-%03d", batch, i),
				Email:  fmt.Sprintf("burst%d-%d@example.com", batch, i),
				Name:   fmt.Sprintf("Burst User %d-%d", batch, i),
			}

			err := client.Publish(ctx, "test.safety", userEvent)
			if err != nil {
				log.Printf("❌ Failed to publish burst user: %v", err)
			} else {
				fmt.Printf("⚡ [%s] Burst user: %s\n",
					time.Now().Format("15:04:05"), userEvent.UserId)
			}
		}

		// Minimal delay for burst effect
		time.Sleep(50 * time.Millisecond)
	}
}
