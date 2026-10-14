package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) UpsertUser(ctx context.Context, email, displayName string) (User, error) {
	var user User
	err := p.pool.QueryRow(ctx, `INSERT INTO users (email, display_name) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name RETURNING id::text, email, display_name, created_at`, email, displayName).Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

func (p *Postgres) RecordSubmission(ctx context.Context, submission Submission) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO submissions (user_id, course_slug, chapter_slug, lesson_slug, status, passed, missing_tests, duration_ms, code) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, submission.UserID, submission.LessonKey.CourseSlug, submission.LessonKey.ChapterSlug, submission.LessonKey.LessonSlug, submission.Status, submission.Passed, nonNilStrings(submission.MissingTests), submission.DurationMs, submission.Code)
	if err != nil {
		return fmt.Errorf("record submission: %w", err)
	}
	return nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (p *Postgres) CompleteLesson(ctx context.Context, userID string, key LessonKey, lessonXP int) (bool, int, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, 0, fmt.Errorf("begin lesson completion: %w", err)
	}
	defer tx.Rollback(ctx)
	var attempts int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE user_id = $1 AND course_slug = $2 AND chapter_slug = $3 AND lesson_slug = $4`, userID, key.CourseSlug, key.ChapterSlug, key.LessonSlug).Scan(&attempts)
	if err != nil {
		return false, 0, fmt.Errorf("count lesson attempts: %w", err)
	}
	if attempts == 0 {
		attempts = 1
	}
	awarded := AwardXP(lessonXP, attempts == 1)
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO progress (user_id, course_slug, chapter_slug, lesson_slug, attempts, xp_awarded) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING RETURNING user_id::text`, userID, key.CourseSlug, key.ChapterSlug, key.LessonSlug, attempts, awarded).Scan(&inserted)
	if err == pgx.ErrNoRows {
		if err := tx.Commit(ctx); err != nil {
			return false, 0, fmt.Errorf("commit existing lesson completion: %w", err)
		}
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("insert lesson progress: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO xp_events (user_id, course_slug, chapter_slug, lesson_slug, amount, reason) VALUES ($1, $2, $3, $4, $5, $6)`, userID, key.CourseSlug, key.ChapterSlug, key.LessonSlug, awarded, "lesson_completed")
	if err != nil {
		return false, 0, fmt.Errorf("record XP event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("commit lesson completion: %w", err)
	}
	return true, awarded, nil
}

func (p *Postgres) ReadProgress(ctx context.Context, userID string) ([]Progress, error) {
	rows, err := p.pool.Query(ctx, `SELECT user_id::text, course_slug, chapter_slug, lesson_slug, completed_at, attempts, xp_awarded FROM progress WHERE user_id = $1 ORDER BY course_slug, chapter_slug, lesson_slug`, userID)
	if err != nil {
		return nil, fmt.Errorf("read progress: %w", err)
	}
	defer rows.Close()
	var results []Progress
	for rows.Next() {
		var progress Progress
		err := rows.Scan(&progress.UserID, &progress.LessonKey.CourseSlug, &progress.LessonKey.ChapterSlug, &progress.LessonKey.LessonSlug, &progress.CompletedAt, &progress.Attempts, &progress.XPAwarded)
		if err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		results = append(results, progress)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progress: %w", err)
	}
	return results, nil
}

func (p *Postgres) TotalXP(ctx context.Context, userID string) (int, error) {
	var total int
	err := p.pool.QueryRow(ctx, `SELECT COALESCE(sum(amount), 0) FROM xp_events WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("read total XP: %w", err)
	}
	return total, nil
}

var _ Store = (*Postgres)(nil)
var _ Store = (*Memory)(nil)
