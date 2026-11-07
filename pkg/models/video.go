package models

import "time"

// VideoStatus は動画の変換ステータスを表す
type VideoStatus string

const (
	StatusPending    VideoStatus = "pending"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusFailed     VideoStatus = "failed"
)

// Video は動画メタデータを表す
type Video struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	OriginalKey string      `json:"original_key"` // MinIO上のオリジナルファイルパス
	HLSKey      string      `json:"hls_key"`      // MinIO上のHLSマスタープレイリストパス
	Status      VideoStatus `json:"status"`
	ErrorMsg    string      `json:"error_msg,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// TranscodeJob はRedisキューに積むジョブを表す
type TranscodeJob struct {
	VideoID     string `json:"video_id"`
	OriginalKey string `json:"original_key"`
}
