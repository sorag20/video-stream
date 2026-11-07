package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/queue"
	"github.com/sorag/video_stream/pkg/storage"
	"github.com/sorag/video_stream/services/upload/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx := context.Background()

	// PostgreSQL
	pool, err := db.NewPool(ctx)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := db.NewVideoRepository(pool)
	if err := repo.Migrate(ctx); err != nil {
		slog.Error("failed to migrate", "err", err)
		os.Exit(1)
	}

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

	h := handler.New(repo, queueClient, storageClient)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	r.Post("/videos", h.Upload)
	r.Get("/videos", h.List)
	r.Get("/videos/{id}", h.Get)

	addr := ":8080"
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Minute, // ボディの読み込みも含むため、最大2GBのアップロードを遅い回線でも受け切れる長さにする
		WriteTimeout:      30 * time.Minute, // ヘッダー受信時点から数えるため、アップロード時間 + MinIO への保存時間を含めて確保する
		IdleTimeout:       120 * time.Second,
	}

	// グレースフルシャットダウン
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("upload service starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
