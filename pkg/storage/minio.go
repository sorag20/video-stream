package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// バケットの論理名。各メソッドにはこの名前を渡し、実際のバケット名への変換は Client が行う
// （S3 ではバケット名が全世界で一意のため、環境変数で指定した名前に置き換える）
const (
	BucketOriginals = "originals"
	BucketHLS       = "hls"
)

// Client はオブジェクトストレージ（ローカルは MinIO、EKS は Amazon S3）の操作を提供する
type Client struct {
	mc      *minio.Client
	buckets map[string]string // 論理名 → 実際のバケット名（MinIO では論理名をそのまま使うため nil）
}

// NewClient はストレージのクライアントを生成する
// STORAGE_BACKEND=s3 のときは Amazon S3 に接続し、それ以外は MinIO に接続する
func NewClient(ctx context.Context) (*Client, error) {
	if os.Getenv("STORAGE_BACKEND") == "s3" {
		return newS3Client(ctx)
	}
	return newMinIOClient(ctx)
}

// newS3Client は Amazon S3 のクライアントを生成する
// 認証は IAM（EKS Pod Identity が Pod に渡す認証情報）を使う。バケットは事前に作成しておき、ここでは存在だけを確かめる
func newS3Client(ctx context.Context) (*Client, error) {
	region := getenv("AWS_REGION", "ap-northeast-1")
	buckets := map[string]string{
		BucketOriginals: os.Getenv("S3_BUCKET_ORIGINALS"),
		BucketHLS:       os.Getenv("S3_BUCKET_HLS"),
	}

	mc, err := minio.New("s3."+region+".amazonaws.com", &minio.Options{
		Creds:  credentials.NewIAM(""),
		Secure: true,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}

	// 設定や権限の誤りに起動時に気づけるよう、バケットにアクセスできるか確かめる
	for name, bucket := range buckets {
		if bucket == "" {
			return nil, fmt.Errorf("bucket name for %q is not set (S3_BUCKET_ORIGINALS / S3_BUCKET_HLS)", name)
		}
		exists, err := mc.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("bucket exists check [%s]: %w", bucket, err)
		}
		if !exists {
			return nil, fmt.Errorf("bucket %s does not exist", bucket)
		}
	}

	return &Client{mc: mc, buckets: buckets}, nil
}

// newMinIOClient は MinIO のクライアントを生成し、必要なバケットを作成する
func newMinIOClient(ctx context.Context) (*Client, error) {
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
	bucket = c.bucketName(bucket)
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
	bucket = c.bucketName(bucket)
	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object [%s/%s]: %w", bucket, key, err)
	}
	return obj, nil
}

// StatObject はオブジェクトのメタデータを取得する
func (c *Client) StatObject(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
	bucket = c.bucketName(bucket)
	info, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return minio.ObjectInfo{}, fmt.Errorf("stat object [%s/%s]: %w", bucket, key, err)
	}
	return info, nil
}

// FGetObject はオブジェクトをローカルファイルとして保存する
func (c *Client) FGetObject(ctx context.Context, bucket, key, localPath string) error {
	bucket = c.bucketName(bucket)
	if err := c.mc.FGetObject(ctx, bucket, key, localPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("fget object [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

// FPutObject はローカルファイルをアップロードする
func (c *Client) FPutObject(ctx context.Context, bucket, key, localPath, contentType string) error {
	bucket = c.bucketName(bucket)
	_, err := c.mc.FPutObject(ctx, bucket, key, localPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("fput object [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

// bucketName は論理名（BucketOriginals / BucketHLS）を実際のバケット名に変換する
func (c *Client) bucketName(name string) string {
	if b, ok := c.buckets[name]; ok {
		return b
	}
	return name
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
