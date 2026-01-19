package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PikaClient wraps Redis client for Pika storage
type PikaClient struct {
	client       *redis.Client
	pipelineSize int
}

// NewPikaClient creates a new Pika client
func NewPikaClient(addr, password string, db, maxConnections, pipelineSize int) (*PikaClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     maxConnections,
		MinIdleConns: maxConnections / 4,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Pika: %w", err)
	}

	return &PikaClient{
		client:       client,
		pipelineSize: pipelineSize,
	}, nil
}

// Get retrieves a value by key
func (p *PikaClient) Get(ctx context.Context, key string) (string, error) {
	return p.client.Get(ctx, key).Result()
}

// Set sets a key-value pair
func (p *PikaClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return p.client.Set(ctx, key, value, ttl).Err()
}

// GetBytes retrieves raw bytes by key
func (p *PikaClient) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return p.client.Get(ctx, key).Bytes()
}

// SetBytes sets raw bytes
func (p *PikaClient) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return p.client.Set(ctx, key, value, ttl).Err()
}

// MGet retrieves multiple keys
func (p *PikaClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return p.client.MGet(ctx, keys...).Result()
}

// Pipeline creates a new pipeline for batch operations
func (p *PikaClient) Pipeline() redis.Pipeliner {
	return p.client.Pipeline()
}

// ExecutePipeline executes a pipeline
func (p *PikaClient) ExecutePipeline(ctx context.Context, pipe redis.Pipeliner) error {
	_, err := pipe.Exec(ctx)
	return err
}

// Del deletes keys
func (p *PikaClient) Del(ctx context.Context, keys ...string) error {
	return p.client.Del(ctx, keys...).Err()
}

// Exists checks if key exists
func (p *PikaClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := p.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Close closes the client connection
func (p *PikaClient) Close() error {
	return p.client.Close()
}

// Keys returns keys matching pattern
func (p *PikaClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return p.client.Keys(ctx, pattern).Result()
}

// Scan iterates over keys matching pattern
func (p *PikaClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys, newCursor, err := p.client.Scan(ctx, cursor, match, count).Result()
	return keys, newCursor, err
}
