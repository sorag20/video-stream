package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	BucketOriginals = "originals"
	BucketHLS       = "hls"
)

// Client はMinIOストレージ操作を提供する
type Client struct {
	mc     *minio.Client
	bucket string // デフォルトバケット（未使用時は各メソッドで指定）
}

// NewClient はMinIOクライアントを生成し、必要なバケットを作成する
func NewClient(ctx context.Context) (*Client, error) {
	endpoint := fmt.Sprintf("%s:%s",
		getenv("MINIO_HOST", "localhost"),
		getenv("MINIO_PORT", "9000"),
	)
	accessKey := getenv("MINIO_ACCESS_KEY", "minioadmin")
	secretKey := getenv("MINIO_SECRET_KEY", "minioadmin")
	useSSL := os.Getenv("MINIO_USE_SSL") == "true"

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}

	client := &Client{mc: mc}

	// 必要なバケットを作成（存在しない場合のみ）
	for _, bucket := range []string{BucketOriginals, BucketHLS} {
		exists, err := mc.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("bucket exists check [%s]: %w", bucket, err)
		}
		if !exists {
			if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, fmt.Errorf("make bucket [%s]: %w", bucket, err)
			}
		}

		// HLSバケットはパブリック読み取りを設定
		if bucket == BucketHLS {
			policy := fmt.Sprintf(`{
				"Version":"2012-10-17",
				"Statement":[{
					"Effect":"Allow",
					"Principal":{"AWS":["*"]},
					"Action":["s3:GetObject"],
					"Resource":["arn:aws:s3:::%s/*"]
				}]
			}`, bucket)
			if err := mc.SetBucketPolicy(ctx, bucket, policy); err != nil {
				return nil, fmt.Errorf("set bucket policy [%s]: %w", bucket, err)
			}
		}
	}

	return client, nil
}

// PutObject はオブジェクトをアップロードする
func (c *Client) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

// GetObject はオブジェクトを取得する
func (c *Client) GetObject(ctx context.Context, bucket, key string) (*minio.Object, error) {
	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object [%s/%s]: %w", bucket, key, err)
	}
	return obj, nil
}

// StatObject はオブジェクトのメタデータを取得する
func (c *Client) StatObject(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
	info, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return minio.ObjectInfo{}, fmt.Errorf("stat object [%s/%s]: %w", bucket, key, err)
	}
	return info, nil
}

// FGetObject はオブジェクトをローカルファイルとして保存する
func (c *Client) FGetObject(ctx context.Context, bucket, key, localPath string) error {
	if err := c.mc.FGetObject(ctx, bucket, key, localPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("fget object [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

// FPutObject はローカルファイルをアップロードする
func (c *Client) FPutObject(ctx context.Context, bucket, key, localPath, contentType string) error {
	_, err := c.mc.FPutObject(ctx, bucket, key, localPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("fput object [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
