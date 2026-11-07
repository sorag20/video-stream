package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/models"
	"github.com/sorag/video_stream/pkg/storage"
)

// Handler はStreaming Serviceのハンドラ
type Handler struct {
	repo    *db.VideoRepository
	storage *storage.Client
}

func New(repo *db.VideoRepository, s *storage.Client) *Handler {
	return &Handler{repo: repo, storage: s}
}

// Playlist はHLSマスタープレイリスト（index.m3u8）を配信する
// GET /stream/{id}/index.m3u8
func (h *Handler) Playlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	video, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("video not found", "id", id, "err", err)
		http.Error(w, "video not found", http.StatusNotFound)
		return
	}

	if video.Status != models.StatusReady {
		http.Error(w, fmt.Sprintf("video is not ready (status: %s)", video.Status), http.StatusAccepted)
		return
	}

	obj, err := h.storage.GetObject(r.Context(), storage.BucketHLS, video.HLSKey)
	if err != nil {
		slog.Error("failed to get playlist", "key", video.HLSKey, "err", err)
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		slog.Error("failed to stat playlist", "key", video.HLSKey, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, obj); err != nil {
		slog.Error("failed to stream playlist", "err", err)
	}
}

// Segment はHLSセグメント（.tsファイル）を配信する
// GET /stream/{id}/{segment}
func (h *Handler) Segment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	segment := chi.URLParam(r, "segment")

	// パストラバーサル防止
	if strings.Contains(segment, "..") || strings.Contains(segment, "/") {
		http.Error(w, "invalid segment name", http.StatusBadRequest)
		return
	}

	ext := filepath.Ext(segment)
	if ext != ".ts" && ext != ".m3u8" {
		http.Error(w, "invalid segment type", http.StatusBadRequest)
		return
	}

	objectKey := fmt.Sprintf("%s/hls/%s", id, segment)

	obj, err := h.storage.GetObject(r.Context(), storage.BucketHLS, objectKey)
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == "NoSuchKey" {
			http.Error(w, "segment not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get segment", "key", objectKey, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		slog.Error("failed to stat segment", "key", objectKey, "err", err)
		http.Error(w, "segment not found", http.StatusNotFound)
		return
	}

	setCORSHeaders(w)
	w.Header().Set("Content-Type", contentTypeForExt(ext))
	// tsセグメントはキャッシュ可能（イミュータブル）
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, obj); err != nil {
		slog.Error("failed to stream segment", "key", objectKey, "err", err)
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/MP2T"
	default:
		return "application/octet-stream"
	}
}
