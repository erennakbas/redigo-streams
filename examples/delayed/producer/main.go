package main

import (
	"context"
	"fmt"
	"github.com/your-username/redigo-streams/examples/proto"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/redigo-streams/pkg/redigo"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Println("📅 [DELAYED PRODUCER] Starting Delayed Task Scheduler...")
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

	// Start the delayed task scheduler
	err = client.StartDelayedScheduler(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to start delayed scheduler: %v", err)
	}
	fmt.Println("✅ Delayed task scheduler started")

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\n📅 Scheduling various delayed tasks...")
	fmt.Println("📋 Will schedule:")
	fmt.Println("   • Immediate tasks (for comparison)")
	fmt.Println("   • Short delays (5-15 seconds)")
	fmt.Println("   • Batch tasks with staggered timing")
	fmt.Println("   • Periodic scheduling demonstration")
	fmt.Println("\n🛑 Press Ctrl+C to stop\n")

	// Start scheduling tasks
	go scheduleDelayedTasks(ctx, client)

	// Statistics monitoring
	go monitorDelayedTasks(ctx, client)

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 Shutting down producer...")
	cancel()
	client.StopDelayedScheduler()
	fmt.Println("✅ Producer stopped cleanly")
}

func scheduleDelayedTasks(ctx context.Context, client *redigo.Client) {
	// Give consumer time to start
	time.Sleep(2 * time.Second)

	counter := 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("\n🔄 Scheduling batch %d...\n", counter)

			// 1. Immediate task (for comparison)
			scheduleImmediateTask(ctx, client, counter)
			time.Sleep(1 * time.Second)

			// 2. Short delayed tasks
			scheduleShortDelayedTasks(ctx, client, counter)
			time.Sleep(1 * time.Second)

			// 3. Scheduled at specific time
			scheduleSpecificTimeTasks(ctx, client, counter)
			time.Sleep(1 * time.Second)

			// 4. Batch tasks with staggered timing
			scheduleBatchTasks(ctx, client, counter)

			counter++

			// Wait before next batch
			fmt.Printf("\n⏱️  Waiting 20 seconds before next batch...\n")
			time.Sleep(20 * time.Second)
		}
	}
}

func scheduleImmediateTask(ctx context.Context, client *redigo.Client, counter int) {
	userEvent := &proto.UserCreatedEvent{
		UserId: fmt.Sprintf("immediate-%d", counter),
		Email:  fmt.Sprintf("immediate%d@example.com", counter),
		Name:   fmt.Sprintf("Immediate User %d", counter),
	}

	err := client.Publish(ctx, "user.events", userEvent)
	if err != nil {
		log.Printf("❌ Failed to publish immediate event: %v", err)
	} else {
		fmt.Printf("📤 [%s] Published immediate event: %s\n",
			time.Now().Format("15:04:05"), userEvent.Name)
	}
}

func scheduleShortDelayedTasks(ctx context.Context, client *redigo.Client, counter int) {
	delays := []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

	for _, delay := range delays {
		delayedUser := &proto.UserCreatedEvent{
			UserId: fmt.Sprintf("delayed-%ds-%d", int(delay.Seconds()), counter),
			Email:  fmt.Sprintf("delayed%ds%d@example.com", int(delay.Seconds()), counter),
			Name:   fmt.Sprintf("%d-Second Delayed User %d", int(delay.Seconds()), counter),
		}

		err := client.PublishDelayed(ctx, "user.events", delayedUser, delay)
		if err != nil {
			log.Printf("❌ Failed to schedule %v delayed event: %v", delay, err)
		} else {
			executeAt := time.Now().Add(delay)
			fmt.Printf("⏰ [%s] Scheduled %v delayed event: %s (will execute at %s)\n",
				time.Now().Format("15:04:05"), delay, delayedUser.Name, executeAt.Format("15:04:05"))
		}
	}
}

func scheduleSpecificTimeTasks(ctx context.Context, client *redigo.Client, counter int) {
	// Schedule email for 20 seconds from now
	emailTask := &proto.EmailSendTask{
		To:      fmt.Sprintf("scheduled%d@example.com", counter),
		Subject: fmt.Sprintf("Scheduled Email #%d", counter),
		Body:    fmt.Sprintf("This email was scheduled for a specific time! Batch %d", counter),
		Variables: map[string]string{
			"batch": fmt.Sprintf("%d", counter),
		},
	}

	scheduleAt := time.Now().Add(20 * time.Second)
	err := client.PublishAt(ctx, "notifications.email", emailTask, scheduleAt)
	if err != nil {
		log.Printf("❌ Failed to schedule email at specific time: %v", err)
	} else {
		fmt.Printf("📧 [%s] Scheduled email for specific time: %s\n",
			time.Now().Format("15:04:05"), scheduleAt.Format("15:04:05"))
	}
}

func scheduleBatchTasks(ctx context.Context, client *redigo.Client, counter int) {
	// Schedule 3 tasks with staggered timing (every 3 seconds)
	for i := 1; i <= 3; i++ {
		delay := time.Duration(i*3) * time.Second
		user := &proto.UserCreatedEvent{
			UserId: fmt.Sprintf("batch-%d-%d", counter, i),
			Email:  fmt.Sprintf("batch%d-%d@example.com", counter, i),
			Name:   fmt.Sprintf("Batch %d User %d", counter, i),
		}

		err := client.PublishDelayed(ctx, "user.events", user, delay)
		if err != nil {
			log.Printf("❌ Failed to schedule batch event %d: %v", i, err)
		} else {
			executeAt := time.Now().Add(delay)
			fmt.Printf("📦 [%s] Scheduled batch %d-%d with %v delay (at %s)\n",
				time.Now().Format("15:04:05"), counter, i, delay, executeAt.Format("15:04:05"))
		}
	}
}

func monitorDelayedTasks(ctx context.Context, client *redigo.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get delayed task statistics
			scheduler, err := client.GetDelayedScheduler()
			if err != nil {
				continue
			}

			stats, err := scheduler.GetStats(ctx)
			if err == nil {
				fmt.Printf("\n📊 [%s] Delayed Task Stats: %+v\n",
					time.Now().Format("15:04:05"), stats)
			}

			// Show pending tasks
			pending, err := scheduler.GetPendingTasks(ctx, 5)
			if err == nil && len(pending) > 0 {
				fmt.Printf("⏳ Pending Tasks (%d):\n", len(pending))
				for _, task := range pending {
					timeUntil := time.Until(task.ScheduledAt)
					if timeUntil > 0 {
						fmt.Printf("  - %s: %s (in %v)\n",
							task.ID[:8], task.Stream, timeUntil.Round(time.Second))
					}
				}
				fmt.Println()
			}
		}
	}
}
