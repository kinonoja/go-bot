package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Memory struct {
	mu           sync.Mutex
	usersByEmail map[string]User
	usersByID    map[string]User
	submissions  []Submission
	progress     map[string]Progress
	nextID       int
}

func NewMemory() *Memory {
	return &Memory{
		usersByEmail: make(map[string]User),
		usersByID:    make(map[string]User),
		progress:     make(map[string]Progress),
	}
}

func (m *Memory) UpsertUser(_ context.Context, email, displayName string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if user, ok := m.usersByEmail[email]; ok {
		user.DisplayName = displayName
		m.usersByEmail[email] = user
		m.usersByID[user.ID] = user
		return user, nil
	}
	m.nextID++
	user := User{ID: fmt.Sprintf("memory-user-%d", m.nextID), Email: email, DisplayName: displayName, CreatedAt: time.Now().UTC()}
	m.usersByEmail[email] = user
	m.usersByID[user.ID] = user
	return user, nil
}

func (m *Memory) RecordSubmission(_ context.Context, submission Submission) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.usersByID[submission.UserID]; !ok {
		return fmt.Errorf("user %q not found", submission.UserID)
	}
	if submission.CreatedAt.IsZero() {
		submission.CreatedAt = time.Now().UTC()
	}
	submission.MissingTests = append([]string(nil), submission.MissingTests...)
	m.submissions = append(m.submissions, submission)
	return nil
}

func (m *Memory) CompleteLesson(_ context.Context, userID string, key LessonKey, lessonXP int) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.usersByID[userID]; !ok {
		return false, 0, fmt.Errorf("user %q not found", userID)
	}
	progressKey := keyFor(userID, key)
	if _, ok := m.progress[progressKey]; ok {
		return false, 0, nil
	}
	attempts := 0
	for _, submission := range m.submissions {
		if submission.UserID == userID && submission.LessonKey == key {
			attempts++
		}
	}
	if attempts == 0 {
		attempts = 1
	}
	awarded := AwardXP(lessonXP, attempts == 1)
	m.progress[progressKey] = Progress{UserID: userID, LessonKey: key, CompletedAt: time.Now().UTC(), Attempts: attempts, XPAwarded: awarded}
	return true, awarded, nil
}

func (m *Memory) ReadProgress(_ context.Context, userID string) ([]Progress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Progress
	for _, progress := range m.progress {
		if progress.UserID == userID {
			result = append(result, progress)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i].LessonKey, result[j].LessonKey
		if a.CourseSlug != b.CourseSlug {
			return a.CourseSlug < b.CourseSlug
		}
		if a.ChapterSlug != b.ChapterSlug {
			return a.ChapterSlug < b.ChapterSlug
		}
		return a.LessonSlug < b.LessonSlug
	})
	return result, nil
}

func (m *Memory) TotalXP(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, progress := range m.progress {
		if progress.UserID == userID {
			total += progress.XPAwarded
		}
	}
	return total, nil
}

func keyFor(userID string, key LessonKey) string {
	return userID + "\x00" + key.CourseSlug + "\x00" + key.ChapterSlug + "\x00" + key.LessonSlug
}
