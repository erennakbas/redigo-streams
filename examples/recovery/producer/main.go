package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/redigo-streams/pkg/proto"
	"github.com/your-username/redigo-streams/pkg/redigo"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("🔄 [RECOVERY PRODUCER] Starting Message Recovery Demo Producer...")
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

	fmt.Println("\n🔄 Publishing messages for recovery testing...")
	fmt.Println("📋 Will publish:")
	fmt.Println("   • Regular user events")
	fmt.Println("   • Problematic messages (to trigger failures)")
	fmt.Println("   • Email tasks for processing")
	fmt.Println("\n💡 Consumer should handle failures and recover messages")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start publishing recovery test messages
	go publishRecoveryTestMessages(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	fmt.Println("✅ Producer stopped cleanly")
}

func publishRecoveryTestMessages(ctx context.Context, client *redigo.Client) {
	// Give consumer time to start
	time.Sleep(2 * time.Second)

	messageCounter := 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("\n📤 Publishing batch %d...\n", messageCounter)

			// 1. Regular message that should succeed
			publishNormalMessage(ctx, client, messageCounter)
			time.Sleep(1 * time.Second)

			// 2. Problematic message that will cause failures
			publishProblematicMessage(ctx, client, messageCounter)
			time.Sleep(1 * time.Second)

			// 3. Email task that should be processed normally
			publishEmailTask(ctx, client, messageCounter)
			time.Sleep(1 * time.Second)

			// 4. Another user with special handling
			publishSpecialUserMessage(ctx, client, messageCounter)

			messageCounter++

			// Wait before next batch
			fmt.Printf("\n⏱️  Waiting 15 seconds before next batch...\n")
			time.Sleep(15 * time.Second)
		}
	}
}

func publishNormalMessage(ctx context.Context, client *redigo.Client, counter int) {
	userEvent := &proto.UserCreatedEvent{
		UserId: fmt.Sprintf("normal-user-%d", counter),
		Email:  fmt.Sprintf("normal%d@example.com", counter),
		Name:   fmt.Sprintf("Normal User %d", counter),
	}

	err := client.Publish(ctx, "test.recovery", userEvent)
	if err != nil {
		log.Printf("❌ Failed to publish normal message: %v", err)
	} else {
		fmt.Printf("✅ [%s] Published normal user: %s\n",
			time.Now().Format("15:04:05"), userEvent.Name)
	}
}

func publishProblematicMessage(ctx context.Context, client *redigo.Client, counter int) {
	// This message will be designed to trigger recovery scenarios
	problematicUser := &proto.UserCreatedEvent{
		UserId: fmt.Sprintf("problematic-user-%d", counter),
		Email:  fmt.Sprintf("problematic%d@example.com", counter),
		Name:   fmt.Sprintf("Problematic User %d", counter),
	}

	err := client.Publish(ctx, "test.recovery", problematicUser)
	if err != nil {
		log.Printf("❌ Failed to publish problematic message: %v", err)
	} else {
		fmt.Printf("⚠️  [%s] Published problematic user: %s (will trigger failures)\n",
			time.Now().Format("15:04:05"), problematicUser.Name)
	}
}

func publishEmailTask(ctx context.Context, client *redigo.Client, counter int) {
	emailTask := &proto.EmailSendTask{
		To:      fmt.Sprintf("recovery%d@example.com", counter),
		Subject: fmt.Sprintf("Recovery Test Email #%d", counter),
		Body:    fmt.Sprintf("This is a test email for recovery scenario %d", counter),
		Variables: map[string]string{
			"batch":    fmt.Sprintf("%d", counter),
			"scenario": "recovery-test",
		},
	}

	err := client.Publish(ctx, "email.tasks", emailTask)
	if err != nil {
		log.Printf("❌ Failed to publish email task: %v", err)
	} else {
		fmt.Printf("📧 [%s] Published email task: %s\n",
			time.Now().Format("15:04:05"), emailTask.Subject)
	}
}

func publishSpecialUserMessage(ctx context.Context, client *redigo.Client, counter int) {
	// Special user that might need multiple attempts
	specialUser := &proto.UserCreatedEvent{
		UserId: fmt.Sprintf("special-user-%d", counter),
		Email:  fmt.Sprintf("special%d@example.com", counter),
		Name:   fmt.Sprintf("Special User %d", counter),
	}

	err := client.Publish(ctx, "test.recovery", specialUser)
	if err != nil {
		log.Printf("❌ Failed to publish special message: %v", err)
	} else {
		fmt.Printf("⭐ [%s] Published special user: %s (may need retry)\n",
			time.Now().Format("15:04:05"), specialUser.Name)
	}
}
