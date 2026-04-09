package queue

import (
	"context"
	"log"
)

// Consumer handles consuming detection tasks
type Consumer struct {
	manager *RabbitMQManager
	handler DetectionHandler
}

// DetectionHandler defines the interface for handling detection tasks
type DetectionHandler interface {
	HandleDetectionTask(ctx context.Context, task *Task) error
}

// NewConsumer creates a new task consumer
func NewConsumer(manager *RabbitMQManager, handler DetectionHandler) *Consumer {
	return &Consumer{
		manager: manager,
		handler: handler,
	}
}

// Start starts consuming tasks
func (c *Consumer) Start(ctx context.Context) error {
	consumerFunc := func(ctx context.Context, task *Task) error {
		log.Printf("Processing task: %s", task.RequestID)
		return c.handler.HandleDetectionTask(ctx, task)
	}

	return c.manager.Consume(ctx, consumerFunc)
}
