package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/models"
	"github.com/sorag/video_stream/pkg/queue"
	"github.com/sorag/video_stream/pkg/storage"
)

const maxUploadSize = 2 << 30 // 2GB

// Handler はUpload Serviceのハンドラ
type Handler struct {
	repo     *db.VideoRepository
	comments *db.CommentRepository
	queue    *queue.Client
	storage  *storage.Client
}

func New(repo *db.VideoRepository, comments *db.CommentRepository, q *queue.Client, s *storage.Client) *Handler {
	return &Handler{repo: repo, comments: comments, queue: q, storage: s}
}

// Upload は動画ファイルを受け取り、MinIOに保存してRedisにジョブを積む
// POST /videos
// Content-Type: multipart/form-data
// フィールド: title (string), file (動画ファイル)
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	// マルチパートの最大サイズを制限
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB をメモリに保持、残りはディスク
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	title := r.FormValue("title")
	if title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		respondError(w, http.StatusRequestEntityTooLarge, "file too large (max 2GB)")
		return
	}

	// 動画IDの生成
	videoID := uuid.New().String()
	originalKey := fmt.Sprintf("%s/original%s", videoID, fileExt(header.Filename))

	// MinIOにアップロード
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.storage.PutObject(r.Context(), storage.BucketOriginals, originalKey, file, header.Size, contentType); err != nil {
		slog.Error("failed to upload to minio", "err", err, "videoID", videoID)
		respondError(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	// DBにメタデータ保存
	now := time.Now()
	video := &models.Video{
		ID:          videoID,
		Title:       title,
		OriginalKey: originalKey,
		Status:      models.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.repo.Create(r.Context(), video); err != nil {
		slog.Error("failed to save video metadata", "err", err, "videoID", videoID)
		respondError(w, http.StatusInternalServerError, "failed to save metadata")
		return
	}

	// Redisにトランスコードジョブを投入
	job := &models.TranscodeJob{
		VideoID:     videoID,
		OriginalKey: originalKey,
	}
	if err := h.queue.Enqueue(r.Context(), job); err != nil {
		slog.Error("failed to enqueue transcode job", "err", err, "videoID", videoID)
		// ジョブ投入失敗時はステータスをfailedに更新
		_ = h.repo.UpdateStatus(r.Context(), videoID, models.StatusFailed, "", "failed to enqueue transcode job")
		respondError(w, http.StatusInternalServerError, "failed to enqueue job")
		return
	}

	slog.Info("video uploaded", "videoID", videoID, "title", title, "size", header.Size)
	respondJSON(w, http.StatusCreated, video)
}

// List は全動画一覧を返す
// GET /videos
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	videos, err := h.repo.List(r.Context())
	if err != nil {
		slog.Error("failed to list videos", "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list videos")
		return
	}
	if videos == nil {
		videos = []*models.Video{} // null ではなく空配列を返す
	}
	respondJSON(w, http.StatusOK, videos)
}

// Get はIDで動画を返す
// GET /videos/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	video, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("failed to get video", "err", err, "id", id)
		respondError(w, http.StatusNotFound, "video not found")
		return
	}

	respondJSON(w, http.StatusOK, video)
}

// --- helpers ---

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "err", err)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// fileExt はファイル名から拡張子を返す（例: ".mp4"）
func fileExt(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' {
			break
		}
	}
	return ".mp4" // デフォルト
}
