package main

import (
	"context"
	"fmt"
	"log"
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

	fmt.Println("🚀 [BASIC PRODUCER] Starting Redis Streams Producer...")
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

	fmt.Println("✅ Producer ready! Publishing messages every 3 seconds...")
	fmt.Println("📋 Messages will be sent to:")
	fmt.Println("   • user.events   - User registration events")
	fmt.Println("   • email.tasks   - Email sending tasks")
	fmt.Println("\n🛑 Press Ctrl+C to stop\n")

	// Start publishing messages
	go publishMessages(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	fmt.Println("✅ Producer stopped cleanly")
}

func publishMessages(ctx context.Context, client *redigo.Client) {
	userCounter := 1

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			timestamp := time.Now()

			// Alternate between user events and email tasks
			if userCounter%2 == 1 {
				publishUserEvent(ctx, client, userCounter, timestamp)
			} else {
				publishEmailTask(ctx, client, userCounter, timestamp)
			}
			userCounter++

			time.Sleep(500 * time.Millisecond)
		}
	}
}

func publishUserEvent(ctx context.Context, client *redigo.Client, counter int, timestamp time.Time) {
	users := []struct {
		name  string
		email string
	}{
		{"John Doe", "john.doe@example.com"},
		{"Jane Smith", "jane.smith@example.com"},
		{"Bob Wilson", "bob.wilson@example.com"},
		{"Alice Brown", "alice.brown@example.com"},
		{"Charlie Davis", "charlie.davis@example.com"},
	}

	user := users[(counter-1)%len(users)]

	userEvent := &proto.UserCreatedEvent{
		UserId: fmt.Sprintf("user-%d", counter),
		Email:  user.email,
		Name:   user.name,
	}

	if err := client.Publish(ctx, "user.events", userEvent); err != nil {
		log.Printf("❌ Failed to publish user event: %v", err)
		return
	}

	fmt.Printf("👤 [%s] Published User Event: %s (%s)\n",
		timestamp.Format("15:04:05"), user.name, user.email)
}

func publishEmailTask(ctx context.Context, client *redigo.Client, counter int, timestamp time.Time) {
	subjects := []string{
		"Welcome to our platform!",
		"Please verify your email",
		"Your order confirmation",
		"Special offer just for you!",
		"Account security notice",
	}

	subject := subjects[(counter-1)%len(subjects)]

	emailTask := &proto.EmailSendTask{
		To:      fmt.Sprintf("user-%d@example.com", counter),
		Subject: subject,
		Body:    fmt.Sprintf("This is an automated email for user-%d", counter),
		Variables: map[string]string{
			"username":  fmt.Sprintf("User-%d", counter),
			"timestamp": timestamp.Format("2006-01-02 15:04:05"),
		},
	}

	if err := client.Publish(ctx, "email.tasks", emailTask); err != nil {
		log.Printf("❌ Failed to publish email task: %v", err)
		return
	}

	fmt.Printf("📧 [%s] Published Email Task: %s\n",
		timestamp.Format("15:04:05"), subject)
}
