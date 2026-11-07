package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/queue"
	"github.com/sorag/video_stream/pkg/storage"
	"github.com/sorag/video_stream/services/transcoder/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// PostgreSQL
	pool, err := db.NewPool(ctx)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := db.NewVideoRepository(pool)

	// Redis
	queueClient, err := queue.NewClient()
	if err != nil {
		slog.Error("failed to connect to redis", "err", err)
		os.Exit(1)
	}
	defer queueClient.Close()

	// MinIO
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		slog.Error("failed to connect to minio", "err", err)
		os.Exit(1)
	}

	w := worker.New(repo, queueClient, storageClient)

	slog.Info("transcoder worker starting")
	w.Run(ctx)
	slog.Info("transcoder worker stopped")
}
