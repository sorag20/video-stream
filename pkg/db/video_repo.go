package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorag/video_stream/pkg/models"
)

// VideoRepository はPostgreSQLへの動画メタデータ操作を提供する
type VideoRepository struct {
	pool *pgxpool.Pool
}

func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

// Migrate はvideos テーブルを作成する（起動時に呼ぶ）
func (r *VideoRepository) Migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS videos (
			id           TEXT PRIMARY KEY,
			title        TEXT NOT NULL,
			original_key TEXT NOT NULL,
			hls_key      TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'pending',
			error_msg    TEXT NOT NULL DEFAULT '',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

// Create は動画レコードを新規作成する
func (r *VideoRepository) Create(ctx context.Context, v *models.Video) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO videos (id, title, original_key, hls_key, status, error_msg, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		v.ID, v.Title, v.OriginalKey, v.HLSKey, v.Status, v.ErrorMsg, v.CreatedAt, v.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}
	return nil
}

// GetByID はIDで動画を取得する
func (r *VideoRepository) GetByID(ctx context.Context, id string) (*models.Video, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, title, original_key, hls_key, status, error_msg, created_at, updated_at
		 FROM videos WHERE id = $1`, id,
	)
	v := &models.Video{}
	if err := row.Scan(&v.ID, &v.Title, &v.OriginalKey, &v.HLSKey, &v.Status, &v.ErrorMsg, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get video by id: %w", err)
	}
	return v, nil
}

// List は全動画を作成日時の降順で取得する
func (r *VideoRepository) List(ctx context.Context) ([]*models.Video, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, original_key, hls_key, status, error_msg, created_at, updated_at
		 FROM videos ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v := &models.Video{}
		if err := rows.Scan(&v.ID, &v.Title, &v.OriginalKey, &v.HLSKey, &v.Status, &v.ErrorMsg, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// UpdateStatus はステータスとHLSキーを更新する
func (r *VideoRepository) UpdateStatus(ctx context.Context, id string, status models.VideoStatus, hlsKey, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE videos SET status=$1, hls_key=$2, error_msg=$3, updated_at=$4 WHERE id=$5`,
		status, hlsKey, errMsg, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}
