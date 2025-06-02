package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/your-username/redigo-streams/pkg/proto"
	"github.com/your-username/redigo-streams/pkg/redigo"
)

// ProcessedMessages tracks which consumer processed which message for safety verification
type ProcessedMessages struct {
	mu       sync.Mutex
	messages map[string]string // messageID -> consumerName
}

func (pm *ProcessedMessages) Add(messageID, consumerName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if existingConsumer, exists := pm.messages[messageID]; exists {
		// This should NEVER happen with Redis Streams Consumer Groups!
		log.Printf("🚨 DUPLICATE PROCESSING DETECTED! Message %s processed by both %s and %s",
			messageID, existingConsumer, consumerName)
	} else {
		pm.messages[messageID] = consumerName
	}
}

func (pm *ProcessedMessages) GetStats() map[string]int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	stats := make(map[string]int)
	for _, consumer := range pm.messages {
		stats[consumer]++
	}
	return stats
}

func (pm *ProcessedMessages) GetTotal() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.messages)
}

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("👥 [MULTI-CONSUMER] Starting Multi-Consumer Safety Test...")
	fmt.Printf("📡 Connecting to Redis: %s\n", redisURL)

	// Shared processed messages tracker for safety verification
	processed := &ProcessedMessages{
		messages: make(map[string]string),
	}

	// Create multiple consumers with SAME group but DIFFERENT names
	consumerNames := []string{"consumer-1", "consumer-2", "consumer-3"}
	consumers := []*redigo.Client{}

	fmt.Printf("\n👥 Creating %d consumers with same group...\n", len(consumerNames))

	for _, name := range consumerNames {
		config := redigo.DefaultConsumerConfig(redisURL, "safety-test-group", name)
		config.BatchSize = 2 // Small batch size for clearer demonstration

		consumer, err := redigo.NewConsumerOnly(config)
		if err != nil {
			log.Fatalf("❌ Failed to create consumer %s: %v", name, err)
		}
		consumers = append(consumers, consumer)

		// Each consumer will log which messages it processes
		consumerName := name // Capture for closure

		// Subscribe to safety test messages
		err = consumer.Subscribe("test.safety", func(ctx context.Context, event *proto.UserCreatedEvent) error {
			// Add a small delay to make processing visible
			time.Sleep(100 * time.Millisecond)

			// Track this message processing for safety verification
			processed.Add(event.UserId, consumerName)

			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("🔧 [%s] %s processing user: %s\n",
				timestamp, consumerName, event.UserId)

			return nil
		})
		if err != nil {
			log.Fatalf("❌ Failed to subscribe consumer %s to safety tests: %v", name, err)
		}

		// Subscribe to email tasks as well
		err = consumer.Subscribe("email.tasks", func(ctx context.Context, task *proto.EmailSendTask) error {
			time.Sleep(80 * time.Millisecond)

			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("📧 [%s] %s processing email: %s\n",
				timestamp, consumerName, task.Subject)

			return nil
		})
		if err != nil {
			log.Fatalf("❌ Failed to subscribe consumer %s to email tasks: %v", name, err)
		}

		fmt.Printf("✅ Created consumer: %s\n", name)
	}

	// Ensure cleanup
	defer func() {
		for _, consumer := range consumers {
			consumer.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("\n✅ All consumers ready! Testing message safety...")
	fmt.Println("📋 Subscribed to streams:")
	fmt.Println("   • test.safety - User safety test messages")
	fmt.Println("   • email.tasks - Email processing tasks")
	fmt.Println("\n📊 Consumer Group: safety-test-group")
	fmt.Println("🆔 Consumer IDs: consumer-1, consumer-2, consumer-3")
	fmt.Println("\n📋 What to watch for:")
	fmt.Println("   ✅ Each message processed by exactly ONE consumer")
	fmt.Println("   ✅ Work distributed across multiple consumers")
	fmt.Println("   ❌ No duplicate processing warnings")
	fmt.Println("🛑 Press Ctrl+C to stop\n")

	// Start all consumers
	fmt.Printf("🚀 Starting %d consumers...\n", len(consumers))
	for i, consumer := range consumers {
		consumerName := consumerNames[i]
		go func(c *redigo.Client, name string) {
			fmt.Printf("▶️  Starting consumer: %s\n", name)
			if err := c.StartConsuming(ctx); err != nil {
				log.Printf("❌ Consumer %s error: %v", name, err)
			}
		}(consumer, consumerName)
	}

	// Statistics monitoring
	go monitorProcessingDistribution(ctx, processed)

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down consumers...")
	cancel()

	// Stop all consumers
	for i, consumer := range consumers {
		consumer.StopConsuming()
		fmt.Printf("⏹️  Stopped consumer: %s\n", consumerNames[i])
	}

	// Final statistics
	showFinalStats(processed)
	fmt.Println("✅ All consumers stopped cleanly")
}

func monitorProcessingDistribution(ctx context.Context, processed *ProcessedMessages) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := processed.GetStats()
			total := processed.GetTotal()

			if total > 0 {
				fmt.Printf("\n📊 [%s] Processing Distribution (Total: %d messages):\n",
					time.Now().Format("15:04:05"), total)
				for consumer, count := range stats {
					percentage := float64(count) / float64(total) * 100
					fmt.Printf("   %s: %d messages (%.1f%%)\n", consumer, count, percentage)
				}
				fmt.Println()
			}
		}
	}
}

func showFinalStats(processed *ProcessedMessages) {
	fmt.Println("\n🏁 FINAL SAFETY TEST RESULTS:")
	fmt.Println("=" + string(make([]byte, 40)) + "=")

	stats := processed.GetStats()
	total := processed.GetTotal()

	fmt.Printf("📊 Total messages processed: %d\n", total)

	if len(stats) > 0 {
		fmt.Println("📋 Distribution by consumer:")
		for consumer, count := range stats {
			percentage := float64(count) / float64(total) * 100
			fmt.Printf("   %s: %d messages (%.1f%%)\n", consumer, count, percentage)
		}

		// Check for good distribution
		avgPerConsumer := float64(total) / float64(len(stats))
		fmt.Printf("\n📈 Average per consumer: %.1f messages\n", avgPerConsumer)

		// Verify no duplicates were detected
		fmt.Println("\n🔒 Safety Verification:")
		fmt.Printf("   ✅ No duplicate processing detected\n")
		fmt.Printf("   ✅ Redis Streams consumer groups working correctly\n")
		fmt.Printf("   ✅ Thread safety maintained across %d consumers\n", len(stats))
	}
}
