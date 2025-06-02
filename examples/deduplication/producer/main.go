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
	"github.com/erennakbas/redigo-streams/pkg/strego"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("📤 [DEDUPLICATION PRODUCER] Starting Deduplication Demo Producer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create publisher
	publisherConfig := strego.DefaultPublisherConfig(redisURL)
	client, err := strego.NewPublisherOnly(publisherConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create publisher: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configure deduplication with 30 minute TTL
	err = client.SetDeduplicationConfig(strego.DeduplicationConfig{
		Enabled:   true,
		KeyPrefix: "dedup_demo",
		TTL:       30 * time.Minute,
	})
	if err != nil {
		log.Fatalf("❌ Failed to configure deduplication: %v", err)
	}
	fmt.Println("✅ Message deduplication enabled with 30-minute TTL")

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\n🧪 Testing various deduplication scenarios...")
	fmt.Println("📋 Will demonstrate:")
	fmt.Println("   • Regular publish (no protection)")
	fmt.Println("   • Content-based deduplication")
	fmt.Println("   • Idempotency key deduplication")
	fmt.Println("   • Business logic deduplication")
	fmt.Println("   • Context-based deduplication")
	fmt.Println("\n🛑 Press Ctrl+C to stop\n")

	// Start the deduplication demonstration
	go runDeduplicationDemo(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	fmt.Println("✅ Producer stopped cleanly")
}

func runDeduplicationDemo(ctx context.Context, client *strego.Client) {
	// Give consumer time to start
	time.Sleep(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Run all deduplication scenarios
			demonstrateRegularPublish(ctx, client)
			time.Sleep(3 * time.Second)

			demonstrateContentDeduplication(ctx, client)
			time.Sleep(3 * time.Second)

			demonstrateIdempotencyKey(ctx, client)
			time.Sleep(3 * time.Second)

			demonstrateBusinessLogic(ctx, client)
			time.Sleep(3 * time.Second)

			demonstrateContextBasedDeduplication(ctx, client)
			time.Sleep(5 * time.Second)

			fmt.Println("\n🔄 Restarting deduplication demo cycle...\n")
		}
	}
}

func demonstrateRegularPublish(ctx context.Context, client *strego.Client) {
	fmt.Println("1️⃣ Regular publish (no deduplication protection):")
	for i := 1; i <= 3; i++ {
		userEvent := &proto.UserCreatedEvent{
			UserId: "user-regular",
			Email:  "regular@example.com",
			Name:   "Regular User",
		}

		err := client.Publish(ctx, "user.events", userEvent)
		if err != nil {
			log.Printf("❌ Failed to publish regular message %d: %v", i, err)
		} else {
			fmt.Printf("📤 [%s] Published regular message %d - WILL PROCESS MULTIPLE TIMES!\n",
				time.Now().Format("15:04:05"), i)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func demonstrateContentDeduplication(ctx context.Context, client *strego.Client) {
	fmt.Println("\n2️⃣ Content-based deduplication:")
	for i := 1; i <= 3; i++ {
		sameContentEvent := &proto.UserCreatedEvent{
			UserId: "user-content-dedup",
			Email:  "content@example.com",
			Name:   "Content Dedup User",
		}

		err := client.PublishWithContentDeduplication(ctx, "user.events", sameContentEvent)
		if err != nil {
			fmt.Printf("🚫 [%s] Attempt %d blocked: %v\n", time.Now().Format("15:04:05"), i, err)
		} else {
			fmt.Printf("✅ [%s] Content dedup message %d published successfully\n",
				time.Now().Format("15:04:05"), i)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func demonstrateIdempotencyKey(ctx context.Context, client *strego.Client) {
	fmt.Println("\n3️⃣ Idempotency key deduplication:")
	idempotencyKey := fmt.Sprintf("user-signup-%d", time.Now().Unix())

	for i := 1; i <= 3; i++ {
		userEvent := &proto.UserCreatedEvent{
			UserId: fmt.Sprintf("user-idem-%d", i), // Different content each time
			Email:  fmt.Sprintf("idem%d@example.com", i),
			Name:   fmt.Sprintf("Idempotency User %d", i),
		}

		err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, idempotencyKey)
		if err != nil {
			fmt.Printf("🚫 [%s] Idempotency attempt %d blocked: %v\n",
				time.Now().Format("15:04:05"), i, err)
		} else {
			fmt.Printf("✅ [%s] Idempotency message %d published successfully\n",
				time.Now().Format("15:04:05"), i)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func demonstrateBusinessLogic(ctx context.Context, client *strego.Client) {
	fmt.Println("\n4️⃣ Business logic deduplication:")
	userID := "user-123"
	action := "account_creation"

	for i := 1; i <= 3; i++ {
		userEvent := &proto.UserCreatedEvent{
			UserId: userID,
			Email:  fmt.Sprintf("business%d@example.com", i), // Different emails
			Name:   fmt.Sprintf("Business User %d", i),       // Different names
		}

		// Generate business logic hash (same user + same action = duplicate)
		businessHash := strego.GenerateBusinessLogicHash("user.events", userID, action)

		err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, businessHash)
		if err != nil {
			fmt.Printf("🚫 [%s] Business logic attempt %d blocked: %v\n",
				time.Now().Format("15:04:05"), i, err)
		} else {
			fmt.Printf("✅ [%s] Business logic message %d published successfully\n",
				time.Now().Format("15:04:05"), i)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func demonstrateContextBasedDeduplication(ctx context.Context, client *strego.Client) {
	fmt.Println("\n5️⃣ Different business contexts (should be allowed):")
	contexts := []string{"web", "mobile", "api"}

	for _, context := range contexts {
		userEvent := &proto.UserCreatedEvent{
			UserId: "user-multi-context",
			Email:  "multi@example.com",
			Name:   "Multi Context User",
		}

		// Different contexts create different hashes
		contextHash := strego.GenerateBusinessLogicHash("user.events", "user-multi-context", "signup", context)

		err := client.PublishWithIdempotencyKey(ctx, "user.events", userEvent, contextHash)
		if err != nil {
			fmt.Printf("🚫 [%s] Context %s blocked: %v\n",
				time.Now().Format("15:04:05"), context, err)
		} else {
			fmt.Printf("✅ [%s] Context %s published successfully\n",
				time.Now().Format("15:04:05"), context)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
