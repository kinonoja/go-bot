package store

import (
	"context"
	"testing"
)

func TestAwardXP(t *testing.T) {
	tests := []struct {
		name  string
		xp    int
		first bool
		want  int
	}{
		{name: "first attempt bonus", xp: 100, first: true, want: 110},
		{name: "rounded down bonus", xp: 19, first: true, want: 20},
		{name: "repeat attempt", xp: 100, first: false, want: 100},
		{name: "nonpositive XP", xp: 0, first: true, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AwardXP(test.xp, test.first); got != test.want {
				t.Fatalf("AwardXP(%d, %t) = %d, want %d", test.xp, test.first, got, test.want)
			}
		})
	}
}

func TestMemoryStoreCompletionAndTotals(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	user, err := store.UpsertUser(ctx, "learner@example.test", "Learner")
	if err != nil {
		t.Fatal(err)
	}
	key := LessonKey{CourseSlug: "go", ChapterSlug: "start", LessonSlug: "hello"}
	if err := store.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: true}); err != nil {
		t.Fatal(err)
	}
	newlyCompleted, awarded, err := store.CompleteLesson(ctx, user.ID, key, 100)
	if err != nil || !newlyCompleted || awarded != 110 {
		t.Fatalf("first completion = (%t, %d, %v), want (true, 110, nil)", newlyCompleted, awarded, err)
	}
	newlyCompleted, awarded, err = store.CompleteLesson(ctx, user.ID, key, 100)
	if err != nil || newlyCompleted || awarded != 0 {
		t.Fatalf("repeat completion = (%t, %d, %v), want (false, 0, nil)", newlyCompleted, awarded, err)
	}
	total, err := store.TotalXP(ctx, user.ID)
	if err != nil || total != 110 {
		t.Fatalf("TotalXP = (%d, %v), want (110, nil)", total, err)
	}
	progress, err := store.ReadProgress(ctx, user.ID)
	if err != nil || len(progress) != 1 || progress[0].XPAwarded != 110 {
		t.Fatalf("ReadProgress = (%+v, %v), want one completed lesson worth 110 XP", progress, err)
	}
}

func TestMemoryStoreAwardAfterEarlierFailure(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	user, _ := store.UpsertUser(ctx, "learner@example.test", "Learner")
	key := LessonKey{CourseSlug: "go", ChapterSlug: "start", LessonSlug: "hello"}
	if err := store.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: false}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSubmission(ctx, Submission{UserID: user.ID, LessonKey: key, Passed: true}); err != nil {
		t.Fatal(err)
	}
	newlyCompleted, awarded, err := store.CompleteLesson(ctx, user.ID, key, 100)
	if err != nil || !newlyCompleted || awarded != 100 {
		t.Fatalf("completion after retry = (%t, %d, %v), want (true, 100, nil)", newlyCompleted, awarded, err)
	}
}
