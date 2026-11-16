package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/models"
	"github.com/sorag/video_stream/pkg/queue"
	"github.com/sorag/video_stream/pkg/storage"
)

// Worker はRedisキューを監視し、トランスコードジョブを処理する
type Worker struct {
	repo      *db.VideoRepository
	queue     *queue.Client
	storage   *storage.Client
	transcoder *Transcoder
}

func New(repo *db.VideoRepository, q *queue.Client, s *storage.Client) *Worker {
	return &Worker{
		repo:      repo,
		queue:     q,
		storage:   s,
		transcoder: NewTranscoder(s),
	}
}

// Run はキューをポーリングし続け、ctxがキャンセルされると停止する
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := w.queue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			slog.Error("dequeue error", "err", err)
			// バックオフして再試行
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}

		if job == nil {
			// タイムアウト（ジョブなし）、ループを継続
			continue
		}

		slog.Info("processing transcode job", "videoID", job.VideoID)
		w.process(ctx, job)
	}
}

// process は1件のジョブを処理する
func (w *Worker) process(ctx context.Context, job *models.TranscodeJob) {
	// ステータスをprocessingに更新
	if err := w.repo.UpdateStatus(ctx, job.VideoID, models.StatusProcessing, "", ""); err != nil {
		if errors.Is(err, db.ErrVideoNotFound) {
			slog.Info("video was deleted before transcoding, skipping", "videoID", job.VideoID)
			return
		}
		slog.Error("failed to update status to processing", "err", err, "videoID", job.VideoID)
		return
	}

	hlsKey, err := w.transcoder.Transcode(ctx, job)
	if err != nil {
		slog.Error("transcode failed", "err", err, "videoID", job.VideoID)
		_ = w.repo.UpdateStatus(ctx, job.VideoID, models.StatusFailed, "", err.Error())
		return
	}

	// ステータスをreadyに更新
	if err := w.repo.UpdateStatus(ctx, job.VideoID, models.StatusReady, hlsKey, ""); err != nil {
		if errors.Is(err, db.ErrVideoNotFound) {
			// 変換中に動画が削除された。アップロード済みの HLS ファイルが残らないよう片付ける
			slog.Info("video was deleted during transcoding, removing hls files", "videoID", job.VideoID)
			if err := w.storage.RemovePrefix(ctx, storage.BucketHLS, job.VideoID+"/"); err != nil {
				slog.Error("failed to remove hls files of deleted video", "err", err, "videoID", job.VideoID)
			}
			return
		}
		slog.Error("failed to update status to ready", "err", err, "videoID", job.VideoID)
		return
	}

	slog.Info("transcode completed", "videoID", job.VideoID, "hlsKey", hlsKey)
}
