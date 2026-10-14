package store

import (
	"context"
	"time"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

type LessonKey struct {
	CourseSlug  string
	ChapterSlug string
	LessonSlug  string
}

type Submission struct {
	UserID       string
	LessonKey    LessonKey
	Status       string
	Passed       bool
	MissingTests []string
	DurationMs   int64
	Code         string
	CreatedAt    time.Time
}

type Progress struct {
	UserID      string
	LessonKey   LessonKey
	CompletedAt time.Time
	Attempts    int
	XPAwarded   int
}

type Store interface {
	UpsertUser(context.Context, string, string) (User, error)
	RecordSubmission(context.Context, Submission) error
	CompleteLesson(context.Context, string, LessonKey, int) (bool, int, error)
	ReadProgress(context.Context, string) ([]Progress, error)
	TotalXP(context.Context, string) (int, error)
}

func AwardXP(lessonXP int, firstAttempt bool) int {
	if lessonXP <= 0 {
		return 0
	}
	if firstAttempt {
		return lessonXP + lessonXP/10
	}
	return lessonXP
}
