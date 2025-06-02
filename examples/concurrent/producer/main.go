package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erennakbas/redigo-streams/examples/proto"
	"github.com/erennakbas/redigo-streams/pkg/redigo"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("🔧 [CONCURRENT PRODUCER] Starting Concurrent Processing Demo Producer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create publisher
	config := redigo.DefaultPublisherConfig(redisURL)
	client, err := redigo.NewPublisherOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create publisher: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\n🔧 Publishing messages for concurrent processing...")
	fmt.Println("📋 Will publish:")
	fmt.Println("   • User events (varying complexity)")
	fmt.Println("   • Email tasks (simulated processing)")
	fmt.Println("   • Batch messages (high volume)")
	fmt.Println("\n💡 Consumer should process these concurrently with multiple workers")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start publishing concurrent test messages
	go publishConcurrentTestMessages(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	fmt.Println("✅ Producer stopped cleanly")
}

func publishConcurrentTestMessages(ctx context.Context, client *redigo.Client) {
	// Give consumer time to start
	time.Sleep(2 * time.Second)

	batchCounter := 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("\n📤 Publishing batch %d...\n", batchCounter)

			// 1. Rapid burst of user events
			publishUserEventsBurst(ctx, client, batchCounter)
			time.Sleep(2 * time.Second)

			// 2. Email tasks mixed in
			publishEmailTasks(ctx, client, batchCounter)
			time.Sleep(2 * time.Second)

			// 3. Heavy load test
			publishHeavyLoad(ctx, client, batchCounter)

			batchCounter++

			// Wait before next batch
			fmt.Printf("\n⏱️  Waiting 10 seconds before next batch...\n")
			time.Sleep(10 * time.Second)
		}
	}
}

func publishUserEventsBurst(ctx context.Context, client *redigo.Client, batch int) {
	fmt.Printf("👥 Publishing user events burst for batch %d...\n", batch)

	// Publish 15 user events rapidly
	for i := 1; i <= 15; i++ {
		userEvent := &proto.UserCreatedEvent{
			UserId: fmt.Sprintf("batch%d-user-%d", batch, i),
			Email:  fmt.Sprintf("batch%d-user%d@example.com", batch, i),
			Name:   fmt.Sprintf("Batch %d User %d", batch, i),
		}

		err := client.Publish(ctx, "test.concurrent", userEvent)
		if err != nil {
			log.Printf("❌ Failed to publish user event %d: %v", i, err)
		} else {
			fmt.Printf("📤 [%s] Published user: %s\n",
				time.Now().Format("15:04:05"), userEvent.Name)
		}

		// Small delay between messages
		time.Sleep(100 * time.Millisecond)
	}
}

func publishEmailTasks(ctx context.Context, client *redigo.Client, batch int) {
	fmt.Printf("📧 Publishing email tasks for batch %d...\n", batch)

	emailTypes := []struct {
		subject  string
		template string
	}{
		{"Welcome aboard!", "Welcome to our platform!"},
		{"Email verification", "Please verify your email address"},
		{"Password reset", "Reset your password"},
		{"Newsletter", "Our weekly newsletter"},
		{"Account update", "Your account has been updated"},
	}

	// Publish 8 email tasks
	for i := 1; i <= 8; i++ {
		emailType := emailTypes[rand.Intn(len(emailTypes))]

		emailTask := &proto.EmailSendTask{
			To:      fmt.Sprintf("batch%d-email%d@example.com", batch, i),
			Subject: fmt.Sprintf("[Batch %d] %s", batch, emailType.subject),
			Body:    fmt.Sprintf("%s (Batch %d, Email %d)", emailType.template, batch, i),
			Variables: map[string]string{
				"batch":     fmt.Sprintf("%d", batch),
				"email_num": fmt.Sprintf("%d", i),
			},
		}

		err := client.Publish(ctx, "test.emails", emailTask)
		if err != nil {
			log.Printf("❌ Failed to publish email task %d: %v", i, err)
		} else {
			fmt.Printf("📧 [%s] Published email: %s\n",
				time.Now().Format("15:04:05"), emailTask.Subject)
		}

		time.Sleep(150 * time.Millisecond)
	}
}

func publishHeavyLoad(ctx context.Context, client *redigo.Client, batch int) {
	fmt.Printf("⚡ Publishing heavy load for batch %d...\n", batch)

	// Publish 25 messages as fast as possible to test concurrent handling
	for i := 1; i <= 25; i++ {
		// Alternate between user events and email tasks
		if i%2 == 0 {
			userEvent := &proto.UserCreatedEvent{
				UserId: fmt.Sprintf("heavy%d-user-%d", batch, i),
				Email:  fmt.Sprintf("heavy%d-user%d@example.com", batch, i),
				Name:   fmt.Sprintf("Heavy Load %d User %d", batch, i),
			}

			err := client.Publish(ctx, "test.concurrent", userEvent)
			if err != nil {
				log.Printf("❌ Failed to publish heavy user event: %v", err)
			} else {
				fmt.Printf("⚡ [%s] Heavy user: %s\n",
					time.Now().Format("15:04:05"), userEvent.Name)
			}
		} else {
			emailTask := &proto.EmailSendTask{
				To:      fmt.Sprintf("heavy%d-load%d@example.com", batch, i),
				Subject: fmt.Sprintf("Heavy Load Test #%d-%d", batch, i),
				Body:    fmt.Sprintf("This is a heavy load test email for batch %d, message %d", batch, i),
			}

			err := client.Publish(ctx, "test.emails", emailTask)
			if err != nil {
				log.Printf("❌ Failed to publish heavy email task: %v", err)
			} else {
				fmt.Printf("⚡ [%s] Heavy email: %s\n",
					time.Now().Format("15:04:05"), emailTask.Subject)
			}
		}

		// Very minimal delay to create load
		time.Sleep(50 * time.Millisecond)
	}
}
