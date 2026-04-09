package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Forwarder 日志转发器接口
type Forwarder interface {
	Forward(ctx context.Context, logEntry map[string]interface{}) error
	Close() error
}

// ElasticsearchForwarder Elasticsearch 日志转发器
type ElasticsearchForwarder struct {
	client       *http.Client
	endpoint     string
	index        string
	username     string
	password     string
	batchSize    int
	batch        []map[string]interface{}
	mu           sync.Mutex
	flushChan    chan struct{}
	logger       *zap.Logger
	flushInterval time.Duration
}

type ElasticsearchConfig struct {
	Endpoint      string
	Index         string
	Username      string
	Password      string
	BatchSize     int
	FlushInterval time.Duration
}

func NewElasticsearchForwarder(config *ElasticsearchConfig, logger *zap.Logger) *ElasticsearchForwarder {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}

	f := &ElasticsearchForwarder{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint:     config.Endpoint,
		index:        config.Index,
		username:     config.Username,
		password:     config.Password,
		batchSize:    config.BatchSize,
		batch:        make([]map[string]interface{}, 0, config.BatchSize),
		flushChan:    make(chan struct{}, 1),
		logger:       logger,
		flushInterval: config.FlushInterval,
	}

	// 启动定时刷新
	go f.startFlushTimer()

	return f
}

func (f *ElasticsearchForwarder) Forward(ctx context.Context, logEntry map[string]interface{}) error {
	f.mu.Lock()
	f.batch = append(f.batch, logEntry)
	shouldFlush := len(f.batch) >= f.batchSize
	f.mu.Unlock()

	if shouldFlush {
		select {
		case f.flushChan <- struct{}{}:
		default:
		}
	}

	return nil
}

func (f *ElasticsearchForwarder) Close() error {
	select {
	case f.flushChan <- struct{}{}:
	default:
	}
	return nil
}

func (f *ElasticsearchForwarder) flush() error {
	f.mu.Lock()
	if len(f.batch) == 0 {
		f.mu.Unlock()
		return nil
	}

	batch := make([]map[string]interface{}, len(f.batch))
	copy(batch, f.batch)
	f.batch = f.batch[:0]
	f.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	// 构建 bulk 请求
	var buf bytes.Buffer
	for _, entry := range batch {
		indexLine := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": f.index,
				"_type":  "_doc",
			},
		}
		indexJSON, _ := json.Marshal(indexLine)
		buf.Write(indexJSON)
		buf.WriteByte('\n')

		entryJSON, _ := json.Marshal(entry)
		buf.Write(entryJSON)
		buf.WriteByte('\n')
	}

	// 发送请求
	req, err := http.NewRequest("POST", f.endpoint+"/_bulk", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if f.username != "" && f.password != "" {
		req.SetBasicAuth(f.username, f.password)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch returned status %d: %s", resp.StatusCode, string(body))
	}

	f.logger.Debug("Forwarded logs to Elasticsearch", zap.Int("count", len(batch)))
	return nil
}

func (f *ElasticsearchForwarder) startFlushTimer() {
	ticker := time.NewTicker(f.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.flush()
		case <-f.flushChan:
			f.flush()
		}
	}
}

// LogstashForwarder Logstash 日志转发器
type LogstashForwarder struct {
	client   *http.Client
	endpoint string
	logger   *zap.Logger
}

func NewLogstashForwarder(endpoint string, logger *zap.Logger) *LogstashForwarder {
	return &LogstashForwarder{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint: endpoint,
		logger:   logger,
	}
}

func (f *LogstashForwarder) Forward(ctx context.Context, logEntry map[string]interface{}) error {
	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("logstash returned status %d", resp.StatusCode)
	}

	return nil
}

func (f *LogstashForwarder) Close() error {
	return nil
}

// AggregateForwarder 聚合多个转发器
type AggregateForwarder struct {
	forwarders []Forwarder
	logger     *zap.Logger
}

func NewAggregateForwarder(forwarders []Forwarder, logger *zap.Logger) *AggregateForwarder {
	return &AggregateForwarder{
		forwarders: forwarders,
		logger:     logger,
	}
}

func (f *AggregateForwarder) Forward(ctx context.Context, logEntry map[string]interface{}) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(f.forwarders))

	for _, forwarder := range f.forwarders {
		wg.Add(1)
		go func(fw Forwarder) {
			defer wg.Done()
			if err := fw.Forward(ctx, logEntry); err != nil {
				f.logger.Error("Failed to forward log", zap.Error(err))
				errChan <- err
			}
		}(forwarder)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("some forwarders failed: %v", errors)
	}

	return nil
}

func (f *AggregateForwarder) Close() error {
	for _, forwarder := range f.forwarders {
		if err := forwarder.Close(); err != nil {
			f.logger.Error("Failed to close forwarder", zap.Error(err))
		}
	}
	return nil
}
