package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn     *amqp.Connection
	Channel  *amqp.Channel
	once     sync.Once
	connLock sync.Mutex
)

// Task represents a detection task
type Task struct {
	RequestID   string `json:"request_id"`
	UserID      uint   `json:"user_id"`
	Content     string `json:"content"`
	ImageURL    string `json:"image_url,omitempty"`
	ContentType string `json:"content_type"`
	Priority    int    `json:"priority"`
}

// ConsumerFunc is the function type for task consumers
type ConsumerFunc func(ctx context.Context, task *Task) error

// Publisher defines the interface for publishing messages
type Publisher interface {
	PublishDetectionRequest(ctx context.Context, req interface{}, priority int) error
}

// RabbitMQManager manages RabbitMQ connection and operations
type RabbitMQManager struct {
	config       *config.RabbitMQConfig
	conn         *amqp.Connection
	channel      *amqp.Channel
	reconnect    bool
	connMutex    sync.RWMutex
	errorHandler func(error)
}

// NewRabbitMQManager creates a new RabbitMQ manager
func NewRabbitMQManager(cfg *config.RabbitMQConfig) (*RabbitMQManager, error) {
	manager := &RabbitMQManager{
		config:    cfg,
		reconnect: true,
	}

	if err := manager.connect(); err != nil {
		return nil, err
	}

	// Start reconnection goroutine
	go manager.monitorConnection()

	return manager, nil
}

// connect establishes a connection to RabbitMQ
func (rm *RabbitMQManager) connect() error {
	rm.connMutex.Lock()
	defer rm.connMutex.Unlock()

	url := fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		rm.config.Username,
		rm.config.Password,
		rm.config.Host,
		rm.config.Port,
		rm.config.Vhost,
	)

	conn, err := amqp.Dial(url)
	if err != nil {
		return errors.ExternalServiceError(err, "Failed to connect to RabbitMQ")
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return errors.ExternalServiceError(err, "Failed to open RabbitMQ channel")
	}

	// Declare exchange
	if err := ch.ExchangeDeclare(
		rm.config.Exchange,
		"direct",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		ch.Close()
		conn.Close()
		return errors.Internal("Failed to declare RabbitMQ exchange").WithDetails(err.Error())
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		rm.config.Queue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return errors.Internal("Failed to declare RabbitMQ queue").WithDetails(err.Error())
	}

	// Bind queue to exchange
	if err := ch.QueueBind(
		q.Name,
		rm.config.RoutingKey,
		rm.config.Exchange,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return errors.Internal("Failed to bind RabbitMQ queue").WithDetails(err.Error())
	}

	rm.conn = conn
	rm.channel = ch
	Conn = conn
	Channel = ch

	log.Println("RabbitMQ connection established successfully")
	return nil
}

// monitorConnection monitors the connection and reconnects if needed
func (rm *RabbitMQManager) monitorConnection() {
	for {
		time.Sleep(5 * time.Second)

		rm.connMutex.Lock()
		connClosed := rm.conn == nil || rm.conn.IsClosed()
		rm.connMutex.Unlock()

		if connClosed {
			if rm.reconnect {
				log.Println("RabbitMQ connection lost, attempting to reconnect...")
				if err := rm.connect(); err != nil {
					log.Printf("Failed to reconnect to RabbitMQ: %v", err)
					time.Sleep(10 * time.Second)
				}
			}
		}
	}
}

// Publish publishes a task to the queue
func (rm *RabbitMQManager) Publish(ctx context.Context, task *Task) error {
	rm.connMutex.RLock()
	ch := rm.channel
	rm.connMutex.RUnlock()

	if ch == nil || ch.IsClosed() {
		return errors.Internal("RabbitMQ channel is closed")
	}

	body, err := json.Marshal(task)
	if err != nil {
		return errors.Internal("Failed to marshal task").WithDetails(err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(
		ctx,
		rm.config.Exchange,
		rm.config.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Priority:     uint8(task.Priority),
		},
	)

	if err != nil {
		return errors.Internal("Failed to publish message to RabbitMQ").WithDetails(err.Error())
	}

	return nil
}

// PublishDetectionRequest publishes a detection request to the queue
func (rm *RabbitMQManager) PublishDetectionRequest(ctx context.Context, req interface{}, priority int) error {
	// Convert req to Task if possible
	task, ok := req.(*Task)
	if !ok {
		return errors.ValidationError("Invalid request type, expected *Task")
	}

	// Override priority if provided
	task.Priority = priority

	return rm.Publish(ctx, task)
}

// Consume starts consuming tasks from the queue
func (rm *RabbitMQManager) Consume(ctx context.Context, handler ConsumerFunc) error {
	rm.connMutex.RLock()
	ch := rm.channel
	rm.connMutex.RUnlock()

	if ch == nil || ch.IsClosed() {
		return errors.Internal("RabbitMQ channel is closed")
	}

	msgs, err := ch.Consume(
		rm.config.Queue,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return errors.Internal("Failed to register RabbitMQ consumer").WithDetails(err.Error())
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Stopping consumer...")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Consumer channel closed")
					return
				}

				var task Task
				if err := json.Unmarshal(msg.Body, &task); err != nil {
					log.Printf("Failed to unmarshal message: %v", err)
					msg.Nack(false, false)
					continue
				}

				if err := handler(ctx, &task); err != nil {
					log.Printf("Failed to process task: %v", err)
					msg.Nack(false, true) // Requeue
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection
func (rm *RabbitMQManager) Close() error {
	rm.connMutex.Lock()
	defer rm.connMutex.Unlock()

	rm.reconnect = false

	if rm.channel != nil && !rm.channel.IsClosed() {
		if err := rm.channel.Close(); err != nil {
			log.Printf("Failed to close channel: %v", err)
		}
	}

	if rm.conn != nil && !rm.conn.IsClosed() {
		if err := rm.conn.Close(); err != nil {
			log.Printf("Failed to close connection: %v", err)
		}
	}

	return nil
}

// HealthCheck checks the RabbitMQ health
func (rm *RabbitMQManager) HealthCheck() error {
	rm.connMutex.RLock()
	defer rm.connMutex.RUnlock()

	if rm.conn == nil || rm.conn.IsClosed() {
		return errors.ExternalServiceError(fmt.Errorf("connection closed"), "RabbitMQ connection is closed")
	}

	return nil
}
