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
	"github.com/erennakbas/redigo-streams/pkg/strego"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("🔧 [CONCURRENT CONSUMER] Starting Concurrent Processing Demo Consumer...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Create concurrent consumer
	config := strego.DefaultConsumerConfig(redisURL, "concurrent-demo", "concurrent-consumer")
	config.BatchSize = 5 // Process 5 messages at a time

	client, err := strego.NewConcurrentConsumerOnly(config)
	if err != nil {
		log.Fatalf("❌ Failed to create concurrent consumer: %v", err)
	}
	defer client.Close()

	// Set worker count for concurrent processing
	err = client.EnableConcurrentProcessing(8) // 8 concurrent workers
	if err != nil {
		log.Fatalf("❌ Failed to enable concurrent processing: %v", err)
	}
	fmt.Println("✅ Concurrent processing enabled with 8 workers")

	// Subscribe to user events with simulated processing complexity
	err = client.SubscribeConcurrent("test.concurrent", func(ctx context.Context, event *proto.UserCreatedEvent) error {
		workerID := ctx.Value("worker_id")
		attempt := ctx.Value("attempt")

		// Simulate different processing times
		processingTime := time.Duration(rand.Intn(2000)) * time.Millisecond

		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("🔧 [%s] Worker %v processing user %s (attempt %v) - est. time: %v\n",
			timestamp, workerID, event.Name, attempt, processingTime)

		time.Sleep(processingTime)

		// Simulate occasional failures (15% failure rate)
		if rand.Float32() < 0.15 {
			fmt.Printf("   ❌ Worker %v failed processing user %s\n\n", workerID, event.Name)
			return fmt.Errorf("simulated processing error for user %s", event.Name)
		}

		fmt.Printf("   ✅ Worker %v completed user %s in %v\n\n",
			workerID, event.Name, processingTime)

		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to user events: %v", err)
	}

	// Subscribe to email tasks
	err = client.SubscribeConcurrent("test.emails", func(ctx context.Context, task *proto.EmailSendTask) error {
		workerID := ctx.Value("worker_id")

		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("📧 [%s] Worker %v sending email to %s: %s\n",
			timestamp, workerID, task.To, task.Subject)

		// Simulate email sending with random processing time
		emailTime := time.Duration(rand.Intn(500)) * time.Millisecond
		time.Sleep(emailTime)

		// Occasional email failures (10% failure rate)
		if rand.Float32() < 0.1 {
			fmt.Printf("   ❌ Worker %v failed to send email to %s\n\n", workerID, task.To)
			return fmt.Errorf("failed to send email to %s", task.To)
		}

		fmt.Printf("   ✅ Worker %v sent email to %s in %v\n\n", workerID, task.To, emailTime)
		return nil
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe to email tasks: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("✅ Consumer ready! Processing messages concurrently...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • test.concurrent - User events")
	fmt.Println("   • test.emails     - Email tasks")
	fmt.Println("\n📊 Consumer Group: concurrent-demo")
	fmt.Println("🆔 Consumer ID: concurrent-consumer")
	fmt.Println("🔧 Workers: 8 concurrent workers")
	fmt.Println("📦 Batch Size: 5 messages")
	fmt.Println("\n💡 Watch the worker IDs to see concurrent processing")
	fmt.Println("⏱️  Notice different processing times per worker")
	fmt.Println("🔄 Failed messages will be retried automatically")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start concurrent consuming
	go func() {
		if err := client.StartConcurrentConsuming(ctx); err != nil {
			log.Printf("❌ Consumer error: %v", err)
		}
	}()

	// Statistics monitoring
	go monitorProcessingStats(ctx)

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down concurrent consumer...")
	cancel()
	client.StopConcurrentConsuming()
	fmt.Println("✅ Consumer stopped cleanly")
}

func monitorProcessingStats(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	messageCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uptime := time.Since(startTime)
			messageCount += 10 // Estimate based on processing

			fmt.Printf("📊 [%s] Processing Stats (Uptime: %v):\n",
				time.Now().Format("15:04:05"), uptime.Truncate(time.Second))
			fmt.Printf("   🔧 Workers: 8 concurrent workers active\n")
			fmt.Printf("   📤 Estimated messages processed: ~%d\n", messageCount)
			fmt.Printf("   ⏱️  Processing time varies per worker\n")
			fmt.Printf("   🔄 Failed messages are automatically retried\n\n")
		}
	}
}
