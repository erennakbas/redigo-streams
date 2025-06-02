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

	fmt.Println("🎯 [DEDUPLICATION CONSUMER] Starting Deduplication Demo Consumer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create consumer
	config := redigo.DefaultConsumerConfig(redisURL, "dedup-demo", "dedup-consumer")
	client, err := redigo.NewConsumerOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	defer client.Close()

	// Subscribe to user events
	err = client.Subscribe("user.events", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("🎯 [%s] Processing user: %s (%s)\n",
			timestamp, event.Name, event.UserId)

		// Simulate processing time
		time.Sleep(100 * time.Millisecond)

		fmt.Printf("   ✅ User processed successfully\n\n")
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✅ Consumer ready! Listening for messages...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • user.events   - User registration events")
	fmt.Println("\n📊 Consumer Group: dedup-demo")
	fmt.Println("🆔 Consumer ID: dedup-consumer")
	fmt.Println("\n📈 Will show deduplication statistics every 10 seconds")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start consuming
	go func() {
		if err := client.StartConsuming(ctx); err != nil {
			log.Printf("❌ Consumer error: %v", err)
		}
	}()

	// Statistics monitoring
	go monitorDeduplicationStats(ctx, client)

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

func monitorDeduplicationStats(ctx context.Context, client *redigo.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	//for {
	//	select {
	//	case <-ctx.Done():
	//		return
	//	case <-ticker.C:
	//		// Get client's publisher to access deduplication stats
	//		publisher, err := redigo.NewPublisherOnly(redigo.DefaultPublisherConfig(
	//			os.Getenv("REDIS_URL")))
	//		if err != nil {
	//			continue
	//		}
	//
	//		deduplicator := publisher.GetDeduplicator()
	//		if deduplicator != nil {
	//			stats, err := deduplicator.GetStats(ctx)
	//			if err == nil {
	//				fmt.Printf("📊 [%s] Deduplication Stats: %+v\n\n",
	//					time.Now().Format("15:04:05"), stats)
	//			}
	//		}
	//		publisher.Close()
	//	}
	//}
}
