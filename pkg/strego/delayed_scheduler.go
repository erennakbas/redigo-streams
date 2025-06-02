package strego

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/erennakbas/redigo-streams/pkg/proto"
)

// DelayedTaskScheduler manages delayed task execution using Redis Sorted Sets
type DelayedTaskScheduler struct {
	client       *redis.Client
	publisher    *RedisPublisher
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	pollInterval time.Duration
	keyPrefix    string
}

// DelayedTask represents a task scheduled for future execution
type DelayedTask struct {
	ID          string            `json:"id"`
	Stream      string            `json:"stream"`
	Message     *anypb.Any        `json:"message"`
	ScheduledAt time.Time         `json:"scheduled_at"`
	CreatedAt   time.Time         `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
	Retries     int               `json:"retries"`
	MaxRetries  int               `json:"max_retries"`
}

// DelayedSchedulerConfig configuration for delayed scheduler
type DelayedSchedulerConfig struct {
	PollInterval time.Duration
	KeyPrefix    string
	BatchSize    int64
	MaxRetries   int
}

// DefaultDelayedSchedulerConfig returns default configuration
func DefaultDelayedSchedulerConfig() DelayedSchedulerConfig {
	return DelayedSchedulerConfig{
		PollInterval: 5 * time.Second,
		KeyPrefix:    "delayed_tasks",
		BatchSize:    100,
		MaxRetries:   3,
	}
}

// NewDelayedTaskScheduler creates a new delayed task scheduler
func NewDelayedTaskScheduler(client *redis.Client, publisher *RedisPublisher, config DelayedSchedulerConfig) *DelayedTaskScheduler {
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "delayed_tasks"
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	return &DelayedTaskScheduler{
		client:       client,
		publisher:    publisher,
		stopCh:       make(chan struct{}),
		pollInterval: config.PollInterval,
		keyPrefix:    config.KeyPrefix,
	}
}

// Start begins the delayed task scheduler
func (d *DelayedTaskScheduler) Start(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return
	}

	d.running = true
	d.wg.Add(1)

	go d.schedulerLoop(ctx)
	log.Printf("Delayed task scheduler started with %v poll interval", d.pollInterval)
}

// Stop stops the delayed task scheduler
func (d *DelayedTaskScheduler) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	d.running = false
	close(d.stopCh)
	d.wg.Wait()
	log.Println("Delayed task scheduler stopped")
}

// ScheduleTask schedules a task for delayed execution
func (d *DelayedTaskScheduler) ScheduleTask(ctx context.Context, stream string, message proto.Message, executeAt time.Time) error {
	// Convert protobuf message to Any
	payloadAny, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("failed to create Any from message: %w", err)
	}

	// Create delayed task
	task := &DelayedTask{
		ID:          d.generateTaskID(),
		Stream:      stream,
		Message:     payloadAny,
		ScheduledAt: executeAt,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]string),
		Retries:     0,
		MaxRetries:  3,
	}

	// Serialize task
	taskData, err := d.serializeTask(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	// Store in Redis Sorted Set with score = execution timestamp
	score := float64(executeAt.Unix())
	delayedSetKey := d.getDelayedSetKey()

	err = d.client.ZAdd(ctx, delayedSetKey, redis.Z{
		Score:  score,
		Member: taskData,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to schedule task: %w", err)
	}

	log.Printf("Scheduled task %s for execution at %v", task.ID, executeAt)
	return nil
}

// ScheduleTaskWithIdempotency schedules a task with custom idempotency key
func (d *DelayedTaskScheduler) ScheduleTaskWithIdempotency(ctx context.Context, stream string, message proto.Message, executeAt time.Time, idempotencyKey string) error {
	// Convert protobuf message to Any
	payloadAny, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("failed to create Any from message: %w", err)
	}

	// Create delayed task with idempotency key
	task := &DelayedTask{
		ID:          d.generateTaskID(),
		Stream:      stream,
		Message:     payloadAny,
		ScheduledAt: executeAt,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]string),
		Retries:     0,
		MaxRetries:  3,
	}

	// Add idempotency key to metadata
	task.Metadata["idempotency_key"] = idempotencyKey

	// Serialize task
	taskData, err := d.serializeTask(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	// Store in Redis Sorted Set with score = execution timestamp
	score := float64(executeAt.Unix())
	delayedSetKey := d.getDelayedSetKey()

	err = d.client.ZAdd(ctx, delayedSetKey, redis.Z{
		Score:  score,
		Member: taskData,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to schedule task with idempotency: %w", err)
	}

	log.Printf("Scheduled task %s with idempotency key %s for execution at %v", task.ID, idempotencyKey, executeAt)
	return nil
}

// schedulerLoop runs the main scheduler loop
func (d *DelayedTaskScheduler) schedulerLoop(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			if err := d.processReadyTasks(ctx); err != nil {
				log.Printf("Error processing ready tasks: %v", err)
			}
		}
	}
}

// processReadyTasks processes tasks that are ready for execution
func (d *DelayedTaskScheduler) processReadyTasks(ctx context.Context) error {
	now := time.Now()
	delayedSetKey := d.getDelayedSetKey()

	// Get ready tasks (score <= current timestamp)
	readyTasks, err := d.client.ZRangeByScore(ctx, delayedSetKey, &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", now.Unix()),
		Offset: 0,
		Count:  100, // Process up to 100 tasks at once
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get ready tasks: %w", err)
	}

	if len(readyTasks) == 0 {
		return nil // No ready tasks
	}

	log.Printf("Processing %d ready tasks", len(readyTasks))

	// Process each ready task
	for _, taskData := range readyTasks {
		task, err := d.deserializeTask(taskData)
		if err != nil {
			log.Printf("Failed to deserialize task: %v", err)
			// Remove invalid task
			d.client.ZRem(ctx, delayedSetKey, taskData)
			continue
		}

		// Execute the task
		if err := d.executeTask(ctx, task); err != nil {
			log.Printf("Failed to execute task %s: %v", task.ID, err)

			// Handle retry logic
			if task.Retries < task.MaxRetries {
				task.Retries++
				// Reschedule with exponential backoff
				retryDelay := time.Duration(math.Pow(2, float64(task.Retries))) * time.Minute
				newExecuteAt := time.Now().Add(retryDelay)

				log.Printf("Retrying task %s in %v (attempt %d/%d)",
					task.ID, retryDelay, task.Retries, task.MaxRetries)

				// Remove from current position and reschedule
				d.client.ZRem(ctx, delayedSetKey, taskData)

				// Update task and reschedule
				if rescheduleErr := d.rescheduleTask(ctx, task, newExecuteAt); rescheduleErr != nil {
					log.Printf("Failed to reschedule task %s: %v", task.ID, rescheduleErr)
				}
			} else {
				log.Printf("Task %s exceeded max retries, moving to failed tasks", task.ID)
				d.moveToFailedTasks(ctx, task)
				d.client.ZRem(ctx, delayedSetKey, taskData)
			}
		} else {
			// Task executed successfully, remove from delayed set
			d.client.ZRem(ctx, delayedSetKey, taskData)
			log.Printf("Successfully executed task %s", task.ID)
		}
	}

	return nil
}

// executeTask executes a delayed task by publishing to stream
func (d *DelayedTaskScheduler) executeTask(ctx context.Context, task *DelayedTask) error {
	// Unmarshal the message
	messageType := task.Message.GetTypeUrl()

	// Initialize metadata map if it's nil
	metadata := make(map[string]string)
	if task.Metadata != nil {
		// Copy existing metadata
		for k, v := range task.Metadata {
			metadata[k] = v
		}
	}

	// Create stream message for execution
	streamMsg := &pb.StreamMessage{
		Id:          task.ID,
		Stream:      task.Stream,
		MessageType: messageType,
		Payload:     task.Message,
		CreatedAt:   timestamppb.New(task.CreatedAt),
		ScheduledAt: timestamppb.New(task.ScheduledAt),
		Metadata:    metadata,
		RetryCount:  int32(task.Retries),
		MaxRetries:  int32(task.MaxRetries),
	}

	// Add execution metadata
	streamMsg.Metadata["delayed_task_id"] = task.ID
	streamMsg.Metadata["execution_time"] = time.Now().Format(time.RFC3339)
	streamMsg.Metadata["scheduled_for"] = task.ScheduledAt.Format(time.RFC3339)

	// Serialize and publish to stream
	data, err := proto.Marshal(streamMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal stream message: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: task.Stream,
		Values: map[string]interface{}{
			"data":            data,
			"type":            messageType,
			"created":         task.CreatedAt.Unix(),
			"scheduled_at":    task.ScheduledAt.Unix(),
			"executed_at":     time.Now().Unix(),
			"delayed_task_id": task.ID,
		},
	}

	return d.client.XAdd(ctx, args).Err()
}

// rescheduleTask reschedules a task for later execution
func (d *DelayedTaskScheduler) rescheduleTask(ctx context.Context, task *DelayedTask, newExecuteAt time.Time) error {
	task.ScheduledAt = newExecuteAt

	taskData, err := d.serializeTask(task)
	if err != nil {
		return fmt.Errorf("failed to serialize rescheduled task: %w", err)
	}

	score := float64(newExecuteAt.Unix())
	delayedSetKey := d.getDelayedSetKey()

	return d.client.ZAdd(ctx, delayedSetKey, redis.Z{
		Score:  score,
		Member: taskData,
	}).Err()
}

// moveToFailedTasks moves a task to failed tasks set
func (d *DelayedTaskScheduler) moveToFailedTasks(ctx context.Context, task *DelayedTask) {
	failedSetKey := d.getFailedSetKey()

	taskData, err := d.serializeTask(task)
	if err != nil {
		log.Printf("Failed to serialize failed task: %v", err)
		return
	}

	// Add to failed tasks with current timestamp as score
	d.client.ZAdd(ctx, failedSetKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: taskData,
	})
}

// GetPendingTasks returns tasks waiting for execution
func (d *DelayedTaskScheduler) GetPendingTasks(ctx context.Context, limit int64) ([]*DelayedTask, error) {
	delayedSetKey := d.getDelayedSetKey()

	if limit <= 0 {
		limit = 100
	}

	taskDataList, err := d.client.ZRange(ctx, delayedSetKey, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}

	tasks := make([]*DelayedTask, 0, len(taskDataList))
	for _, taskData := range taskDataList {
		task, err := d.deserializeTask(taskData)
		if err != nil {
			log.Printf("Failed to deserialize pending task: %v", err)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetFailedTasks returns failed tasks
func (d *DelayedTaskScheduler) GetFailedTasks(ctx context.Context, limit int64) ([]*DelayedTask, error) {
	failedSetKey := d.getFailedSetKey()

	if limit <= 0 {
		limit = 100
	}

	taskDataList, err := d.client.ZRange(ctx, failedSetKey, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get failed tasks: %w", err)
	}

	tasks := make([]*DelayedTask, 0, len(taskDataList))
	for _, taskData := range taskDataList {
		task, err := d.deserializeTask(taskData)
		if err != nil {
			log.Printf("Failed to deserialize failed task: %v", err)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetStats returns scheduler statistics
func (d *DelayedTaskScheduler) GetStats(ctx context.Context) (map[string]interface{}, error) {
	delayedSetKey := d.getDelayedSetKey()
	failedSetKey := d.getFailedSetKey()

	// Get counts
	pendingCount, err := d.client.ZCard(ctx, delayedSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending count: %w", err)
	}

	failedCount, err := d.client.ZCard(ctx, failedSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get failed count: %w", err)
	}

	// Get next execution time
	var nextExecution *time.Time
	next, err := d.client.ZRangeByScoreWithScores(ctx, delayedSetKey, &redis.ZRangeBy{
		Min:   "0",
		Max:   "+inf",
		Count: 1,
	}).Result()
	if err == nil && len(next) > 0 {
		nextTime := time.Unix(int64(next[0].Score), 0)
		nextExecution = &nextTime
	}

	d.mu.RLock()
	running := d.running
	d.mu.RUnlock()

	stats := map[string]interface{}{
		"running":        running,
		"pending_tasks":  pendingCount,
		"failed_tasks":   failedCount,
		"poll_interval":  d.pollInterval.String(),
		"next_execution": nextExecution,
	}

	return stats, nil
}

// Helper methods

func (d *DelayedTaskScheduler) getDelayedSetKey() string {
	return fmt.Sprintf("%s:delayed", d.keyPrefix)
}

func (d *DelayedTaskScheduler) getFailedSetKey() string {
	return fmt.Sprintf("%s:failed", d.keyPrefix)
}

func (d *DelayedTaskScheduler) generateTaskID() string {
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), time.Now().UnixNano()%10000)
}

func (d *DelayedTaskScheduler) serializeTask(task *DelayedTask) (string, error) {
	// Convert our internal DelayedTask to protobuf DelayedTask
	pbTask := &pb.DelayedTask{
		Id:         task.ID,
		Stream:     task.Stream,
		Payload:    task.Message,
		ExecuteAt:  timestamppb.New(task.ScheduledAt),
		Metadata:   task.Metadata,
		RetryCount: int32(task.Retries),
		MaxRetries: int32(task.MaxRetries),
	}

	// Serialize to protobuf bytes
	data, err := proto.Marshal(pbTask)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf task: %w", err)
	}

	// Return as base64 string for Redis storage
	return string(data), nil
}

func (d *DelayedTaskScheduler) deserializeTask(data string) (*DelayedTask, error) {
	// Deserialize protobuf DelayedTask
	pbTask := &pb.DelayedTask{}
	if err := proto.Unmarshal([]byte(data), pbTask); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf task: %w", err)
	}

	// Convert protobuf DelayedTask to internal DelayedTask
	task := &DelayedTask{
		ID:          pbTask.GetId(),
		Stream:      pbTask.GetStream(),
		Message:     pbTask.GetPayload(),
		ScheduledAt: pbTask.GetExecuteAt().AsTime(),
		CreatedAt:   time.Now(), // We don't store created_at in protobuf version
		Metadata:    pbTask.GetMetadata(),
		Retries:     int(pbTask.GetRetryCount()),
		MaxRetries:  int(pbTask.GetMaxRetries()),
	}

	return task, nil
}
