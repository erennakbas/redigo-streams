package redigo

import "errors"

// Common errors
var (
	ErrPublisherNotAvailable = errors.New("publisher not available")
	ErrConsumerNotAvailable  = errors.New("consumer not available")
	ErrConsumerRunning       = errors.New("consumer is already running")
	ErrConsumerStopped       = errors.New("consumer is stopped")
	ErrInvalidHandler        = errors.New("invalid handler function")
	ErrInvalidMessage        = errors.New("invalid message type")
	ErrMaxRetriesExceeded    = errors.New("maximum retries exceeded")
	ErrStreamNotFound        = errors.New("stream not found")
	ErrConsumerGroupExists   = errors.New("consumer group already exists")
)
