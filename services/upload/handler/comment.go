package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sorag/video_stream/pkg/db"
	"github.com/sorag/video_stream/pkg/models"
)

const (
	maxCommentBodyLen   = 1000     // 文字数 (rune)
	maxCommentAuthorLen = 30       // 文字数 (rune)
	maxCommentReqSize   = 16 << 10 // リクエストボディの上限 (16KB)
	defaultAuthor       = "匿名"
)

type createCommentRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// ListComments は動画のコメント一覧を新しい順で返す
// GET /videos/{id}/comments
func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "id")

	if _, err := h.repo.GetByID(r.Context(), videoID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "video not found")
			return
		}
		slog.Error("failed to get video", "err", err, "videoID", videoID)
		respondError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}

	comments, err := h.comments.ListByVideoID(r.Context(), videoID)
	if err != nil {
		slog.Error("failed to list comments", "err", err, "videoID", videoID)
		respondError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}
	respondJSON(w, http.StatusOK, comments)
}

// CreateComment は動画にコメントを投稿する
// POST /videos/{id}/comments
// Content-Type: application/json
// ボディ: {"author": "名前（省略時は「匿名」）", "body": "コメント本文"}
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "id")

	var req createCommentRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxCommentReqSize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}
	if utf8.RuneCountInString(body) > maxCommentBodyLen {
		respondError(w, http.StatusBadRequest, "body is too long (max 1000 characters)")
		return
	}

	author := strings.TrimSpace(req.Author)
	if author == "" {
		author = defaultAuthor
	}
	if utf8.RuneCountInString(author) > maxCommentAuthorLen {
		respondError(w, http.StatusBadRequest, "author is too long (max 30 characters)")
		return
	}

	comment := &models.Comment{
		ID:        uuid.New().String(),
		VideoID:   videoID,
		Author:    author,
		Body:      body,
		CreatedAt: time.Now(),
	}
	if err := h.comments.Create(r.Context(), comment); err != nil {
		if errors.Is(err, db.ErrVideoNotFound) {
			respondError(w, http.StatusNotFound, "video not found")
			return
		}
		slog.Error("failed to save comment", "err", err, "videoID", videoID)
		respondError(w, http.StatusInternalServerError, "failed to save comment")
		return
	}

	respondJSON(w, http.StatusCreated, comment)
}
