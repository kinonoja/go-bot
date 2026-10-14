package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var sharedStoreTestID atomic.Uint64

func runStoreBehaviorSuite(t *testing.T, dataStore Store) {
	t.Helper()
	tests := []struct {
		name        string
		submissions []bool
		wantXP      int
	}{
		{name: "first completion awards XP", submissions: []bool{false, false, true}, wantXP: 100},
		{name: "first attempt bonus", submissions: []bool{true}, wantXP: 110},
		{name: "completion after failed attempt has no bonus", submissions: []bool{false, true}, wantXP: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			user, key := newSuiteUserAndKey(t, dataStore)
			for _, passed := range test.submissions {
				if err := dataStore.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: passed}); err != nil {
					t.Fatalf("record submission: %v", err)
				}
			}
			newlyCompleted, awarded, err := dataStore.CompleteLesson(ctx, user.ID, key, 100)
			if err != nil || !newlyCompleted || awarded != test.wantXP {
				t.Fatalf("CompleteLesson() = (%t, %d, %v), want (true, %d, nil)", newlyCompleted, awarded, err, test.wantXP)
			}
			assertStoredCompletion(t, dataStore, ctx, user.ID, key, test.wantXP)
		})
	}
	t.Run("repeat completion awards zero", func(t *testing.T) {
		ctx := context.Background()
		user, key := newSuiteUserAndKey(t, dataStore)
		if err := dataStore.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: true}); err != nil {
			t.Fatalf("record first submission: %v", err)
		}
		first, firstXP, err := dataStore.CompleteLesson(ctx, user.ID, key, 100)
		if err != nil || !first || firstXP != 110 {
			t.Fatalf("first completion = (%t, %d, %v), want (true, 110, nil)", first, firstXP, err)
		}
		repeated, repeatedXP, err := dataStore.CompleteLesson(ctx, user.ID, key, 100)
		if err != nil || repeated || repeatedXP != 0 {
			t.Fatalf("repeat completion = (%t, %d, %v), want (false, 0, nil)", repeated, repeatedXP, err)
		}
		assertStoredCompletion(t, dataStore, ctx, user.ID, key, 110)
	})
	t.Run("concurrent completion awards XP once", func(t *testing.T) {
		ctx := context.Background()
		user, key := newSuiteUserAndKey(t, dataStore)
		if err := dataStore.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: true}); err != nil {
			t.Fatalf("record submission: %v", err)
		}
		type result struct {
			newlyCompleted bool
			xp             int
			err            error
		}
		start := make(chan struct{})
		results := make(chan result, 2)
		var wait sync.WaitGroup
		for range 2 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				newlyCompleted, xp, err := dataStore.CompleteLesson(ctx, user.ID, key, 100)
				results <- result{newlyCompleted: newlyCompleted, xp: xp, err: err}
			}()
		}
		close(start)
		wait.Wait()
		close(results)
		newCompletions := 0
		awardedTotal := 0
		for result := range results {
			if result.err != nil {
				t.Fatalf("concurrent CompleteLesson(): %v", result.err)
			}
			if result.newlyCompleted {
				newCompletions++
			}
			awardedTotal += result.xp
		}
		if newCompletions != 1 || awardedTotal != 110 {
			t.Fatalf("concurrent results = (%d new, %d XP), want (1 new, 110 XP)", newCompletions, awardedTotal)
		}
		assertStoredCompletion(t, dataStore, ctx, user.ID, key, 110)
	})
}

func newSuiteUserAndKey(t *testing.T, dataStore Store) (User, LessonKey) {
	t.Helper()
	id := sharedStoreTestID.Add(1)
	ctx := context.Background()
	user, err := dataStore.UpsertUser(ctx, fmt.Sprintf("store-suite-%d-%d@example.test", time.Now().UnixNano(), id), "Store Suite")
	if err != nil {
		t.Fatalf("upsert suite user: %v", err)
	}
	key := LessonKey{CourseSlug: "store-suite", ChapterSlug: "behavior", LessonSlug: fmt.Sprintf("lesson-%d", id)}
	return user, key
}

func assertStoredCompletion(t *testing.T, dataStore Store, ctx context.Context, userID string, key LessonKey, wantXP int) {
	t.Helper()
	progress, err := dataStore.ReadProgress(ctx, userID)
	if err != nil {
		t.Fatalf("ReadProgress(): %v", err)
	}
	if len(progress) != 1 || progress[0].LessonKey != key || progress[0].XPAwarded != wantXP {
		t.Fatalf("ReadProgress() = %+v, want one lesson with %d XP", progress, wantXP)
	}
	totalXP, err := dataStore.TotalXP(ctx, userID)
	if err != nil || totalXP != wantXP {
		t.Fatalf("TotalXP() = (%d, %v), want (%d, nil)", totalXP, err, wantXP)
	}
}

func TestMemoryStoreSharedBehavior(t *testing.T) {
	runStoreBehaviorSuite(t, NewMemory())
}
