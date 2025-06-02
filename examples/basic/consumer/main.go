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

	fmt.Println("🎯 [BASIC CONSUMER] Starting Redis Streams Consumer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create consumer
	config := redigo.DefaultConsumerConfig(redisURL, "basic-group", "basic-consumer-1")
	client, err := redigo.NewConsumerOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	defer client.Close()

	// Subscribe to user events
	err = client.Subscribe("user.events", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("👤 [%s] Processing User Event: %s (%s)\n",
			timestamp, event.Name, event.Email)

		// Simulate user processing time
		time.Sleep(200 * time.Millisecond)

		fmt.Printf("   ✅ User %s registered successfully\n\n", event.Name)
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to user events: %v", err)
	}

	// Subscribe to email tasks
	err = client.Subscribe("email.tasks", func(ctx context.Context, task *proto.EmailSendTask) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("📧 [%s] Processing Email Task: %s\n",
			timestamp, task.Subject)
		fmt.Printf("   📬 Sending to: %s\n", task.To)

		// Simulate email sending time
		time.Sleep(300 * time.Millisecond)

		fmt.Printf("   ✅ Email sent successfully\n\n")
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to email tasks: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✅ Consumer ready! Listening for messages...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • user.events   - User registration events")
	fmt.Println("   • email.tasks   - Email sending tasks")
	fmt.Println("\n📊 Consumer Group: basic-group")
	fmt.Println("🆔 Consumer ID: basic-consumer-1")
	fmt.Println("\n🛑 Press Ctrl+C to stop\n")

	// Start consuming in a goroutine
	go func() {
		if err := client.StartConsuming(ctx); err != nil {
			log.Printf("❌ Consumer error: %v", err)
		}
	}()

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Message statistics
	go printStats(ctx)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down consumer...")
	cancel()
	client.StopConsuming()
	fmt.Println("✅ Consumer stopped cleanly")
}

func printStats(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uptime := time.Since(startTime)
			fmt.Printf("📊 [%s] Consumer Stats - Uptime: %v\n\n",
				time.Now().Format("15:04:05"), uptime.Truncate(time.Second))
		}
	}
}
