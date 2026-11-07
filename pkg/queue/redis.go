package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sorag/video_stream/pkg/models"
)

const transcodeQueueKey = "transcode:jobs"

// Client はRedisキューの操作を提供する
type Client struct {
	rdb *redis.Client
}

// NewClient はRedisクライアントを生成する
func NewClient() (*Client, error) {
	addr := fmt.Sprintf("%s:%s",
		getenv("REDIS_HOST", "localhost"),
		getenv("REDIS_PORT", "6379"),
	)
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Enqueue はトランスコードジョブをキューに追加する
func (c *Client) Enqueue(ctx context.Context, job *models.TranscodeJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return c.rdb.RPush(ctx, transcodeQueueKey, data).Err()
}

// Dequeue はキューからジョブをブロッキングで取得する（タイムアウト付き）
func (c *Client) Dequeue(ctx context.Context, timeout time.Duration) (*models.TranscodeJob, error) {
	result, err := c.rdb.BLPop(ctx, timeout, transcodeQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // タイムアウト、ジョブなし
		}
		return nil, fmt.Errorf("blpop: %w", err)
	}

	// result[0] = キー名, result[1] = 値
	var job models.TranscodeJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return &job, nil
}

// Close はRedis接続を閉じる
func (c *Client) Close() error {
	return c.rdb.Close()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
