package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorag/video_stream/pkg/models"
)

// ErrVideoNotFound はコメント対象の動画が存在しないことを表す
var ErrVideoNotFound = errors.New("video not found")

// PostgreSQL の外部キー制約違反 (foreign_key_violation)
const pgForeignKeyViolation = "23503"

// CommentRepository はPostgreSQLへのコメント操作を提供する
type CommentRepository struct {
	pool *pgxpool.Pool
}

func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{pool: pool}
}

// Migrate はcomments テーブルを作成する（起動時に呼ぶ。videos テーブルを参照するため VideoRepository.Migrate の後に呼ぶこと）
func (r *CommentRepository) Migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS comments (
			id         TEXT PRIMARY KEY,
			video_id   TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
			author     TEXT NOT NULL,
			body       TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS comments_video_id_created_at_idx ON comments (video_id, created_at DESC);
	`)
	return err
}

// Create はコメントを新規作成する。動画が存在しない場合は ErrVideoNotFound を返す
func (r *CommentRepository) Create(ctx context.Context, c *models.Comment) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO comments (id, video_id, author, body, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.VideoID, c.Author, c.Body, c.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return ErrVideoNotFound
		}
		return fmt.Errorf("insert comment: %w", err)
	}
	return nil
}

// ListByVideoID は動画のコメントを作成日時の降順で取得する
func (r *CommentRepository) ListByVideoID(ctx context.Context, videoID string) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, video_id, author, body, created_at
		 FROM comments WHERE video_id = $1 ORDER BY created_at DESC, id`, videoID,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := []*models.Comment{} // JSON で null ではなく空配列になるようにする
	for rows.Next() {
		c := &models.Comment{}
		if err := rows.Scan(&c.ID, &c.VideoID, &c.Author, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
