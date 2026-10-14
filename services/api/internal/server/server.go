package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kinonoja/gobot/services/api/internal/lessons"
	"github.com/kinonoja/gobot/services/api/internal/runnerclient"
	"github.com/kinonoja/gobot/services/api/internal/store"
)

const maxSubmitBody = 100 * 1024
const userSubmitInterval = 2 * time.Second

type Runner interface {
	Run(context.Context, runnerclient.Request) (runnerclient.Response, error)
}

type Handler struct {
	courses []lessons.Course
	store   store.Store
	runner  Runner
	devAuth bool
	rateMu  sync.Mutex
	lastRun map[string]time.Time
}

type submitRequest struct {
	Files map[string]string `json:"files"`
}

type submitTest struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Output string `json:"output,omitempty"`
}

type submitResponse struct {
	Passed         bool         `json:"passed"`
	Status         string       `json:"status"`
	Tests          []submitTest `json:"tests"`
	MissingTests   []string     `json:"missing_tests"`
	Stderr         string       `json:"stderr,omitempty"`
	XPAwarded      int          `json:"xp_awarded"`
	NewlyCompleted bool         `json:"newly_completed"`
	DurationMs     int64        `json:"duration_ms"`
}

type lessonResponse struct {
	Slug       string            `json:"slug"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Starter    map[string]string `json:"starter_files"`
	XP         int               `json:"xp"`
	Difficulty string            `json:"difficulty"`
}

type progressResponse struct {
	Completed []progressLesson `json:"completed_lessons"`
	TotalXP   int              `json:"total_xp"`
}

type progressLesson struct {
	CourseSlug  string    `json:"course_slug"`
	ChapterSlug string    `json:"chapter_slug"`
	LessonSlug  string    `json:"lesson_slug"`
	CompletedAt time.Time `json:"completed_at"`
	XPAwarded   int       `json:"xp_awarded"`
}

func NewHandler(courses []lessons.Course, dataStore store.Store, runner Runner, devAuth bool) http.Handler {
	handler := &Handler{courses: courses, store: dataStore, runner: runner, devAuth: devAuth, lastRun: make(map[string]time.Time)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /api/courses", handler.listCourses)
	mux.HandleFunc("GET /api/courses/{course}/lessons/{chapter}/{lesson}", handler.getLesson)
	mux.HandleFunc("POST /api/courses/{course}/lessons/{chapter}/{lesson}/submit", handler.submit)
	mux.HandleFunc("GET /api/me/progress", handler.getProgress)
	return handler.middleware(mux)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listCourses(w http.ResponseWriter, _ *http.Request) {
	public := make([]lessons.PublicCourse, 0, len(h.courses))
	for _, course := range h.courses {
		public = append(public, course.PublicView())
	}
	writeJSON(w, http.StatusOK, public)
}

func (h *Handler) getLesson(w http.ResponseWriter, r *http.Request) {
	_, _, lesson, ok := h.findLesson(r.PathValue("course"), r.PathValue("chapter"), r.PathValue("lesson"))
	if !ok {
		writeError(w, http.StatusNotFound, "lesson not found")
		return
	}
	writeJSON(w, http.StatusOK, lessonResponse{Slug: lesson.Slug, Title: lesson.Title, Body: lesson.Body, Starter: lesson.StarterFiles, XP: lesson.XP, Difficulty: lesson.Difficulty})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	if h.isRateLimited(user.ID, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "submit rate limit exceeded")
		return
	}
	var request submitRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	if len(request.Files) == 0 {
		writeError(w, http.StatusBadRequest, "files must contain at least one Go file")
		return
	}
	course, chapter, lesson, ok := h.findLesson(r.PathValue("course"), r.PathValue("chapter"), r.PathValue("lesson"))
	if !ok {
		writeError(w, http.StatusNotFound, "lesson not found")
		return
	}
	key := store.LessonKey{CourseSlug: course.Slug, ChapterSlug: chapter.Slug, LessonSlug: lesson.Slug}
	runnerResponse, err := h.runner.Run(r.Context(), runnerclient.Request{
		Files: request.Files, GraderFiles: lesson.GraderFilesForInternalUse(), ExpectedTests: lesson.ExpectedTests,
		Mode: "test", TimeoutMs: 30000,
	})
	if err != nil {
		if errors.Is(err, runnerclient.ErrRateLimited) {
			w.Header().Set("Retry-After", "2")
			writeError(w, http.StatusServiceUnavailable, "runner is busy; retry shortly")
			return
		}
		writeError(w, http.StatusBadGateway, "runner is unavailable")
		return
	}
	passed := runnerResponse.Status == "ok" && len(runnerResponse.MissingTests) == 0 && expectedPassed(lesson.ExpectedTests, runnerResponse.Tests)
	code, err := json.Marshal(request.Files)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not record submission")
		return
	}
	if err := h.store.RecordSubmission(r.Context(), store.Submission{
		UserID: user.ID, LessonKey: key, Status: runnerResponse.Status, Passed: passed,
		MissingTests: runnerResponse.MissingTests, DurationMs: runnerResponse.DurationMs, Code: string(code),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record submission")
		return
	}
	newlyCompleted := false
	xpAwarded := 0
	if passed {
		newlyCompleted, xpAwarded, err = h.store.CompleteLesson(r.Context(), user.ID, key, lesson.XP)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not update lesson progress")
			return
		}
	}
	tests := make([]submitTest, 0, len(runnerResponse.Tests))
	for _, result := range runnerResponse.Tests {
		item := submitTest{Name: result.Name, Passed: result.Passed}
		if !result.Passed {
			item.Output = truncateRunes(result.Output, 2000)
		}
		tests = append(tests, item)
	}
	response := submitResponse{
		Passed: passed, Status: runnerResponse.Status, Tests: tests,
		MissingTests: append([]string{}, runnerResponse.MissingTests...), XPAwarded: xpAwarded,
		NewlyCompleted: newlyCompleted, DurationMs: runnerResponse.DurationMs,
	}
	if runnerResponse.Status == "compile_error" {
		response.Stderr = truncateRunes(runnerResponse.Stderr, 4000)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	progress, err := h.store.ReadProgress(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read progress")
		return
	}
	totalXP, err := h.store.TotalXP(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read progress")
		return
	}
	completed := make([]progressLesson, 0, len(progress))
	for _, item := range progress {
		completed = append(completed, progressLesson{CourseSlug: item.LessonKey.CourseSlug, ChapterSlug: item.LessonKey.ChapterSlug, LessonSlug: item.LessonKey.LessonSlug, CompletedAt: item.CompletedAt, XPAwarded: item.XPAwarded})
	}
	writeJSON(w, http.StatusOK, progressResponse{Completed: completed, TotalXP: totalXP})
}

func (h *Handler) authenticatedUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	if !h.devAuth {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return store.User{}, false
	}
	email := strings.TrimSpace(r.Header.Get("X-Dev-User-Email"))
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return store.User{}, false
	}
	displayName := strings.SplitN(email, "@", 2)[0]
	user, err := h.store.UpsertUser(r.Context(), email, displayName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not authenticate user")
		return store.User{}, false
	}
	return user, true
}

func (h *Handler) isRateLimited(userID string, now time.Time) bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	if last, exists := h.lastRun[userID]; exists && now.Sub(last) < userSubmitInterval {
		return true
	}
	h.lastRun[userID] = now
	return false
}

func (h *Handler) findLesson(courseSlug, chapterSlug, lessonSlug string) (lessons.Course, lessons.Chapter, lessons.Lesson, bool) {
	for _, course := range h.courses {
		if course.Slug != courseSlug {
			continue
		}
		for _, chapter := range course.Chapters {
			if chapter.Slug != chapterSlug {
				continue
			}
			for _, lesson := range chapter.Lessons {
				if lesson.Slug == lessonSlug {
					return course, chapter, lesson, true
				}
			}
		}
	}
	return lessons.Course{}, lessons.Chapter{}, lessons.Lesson{}, false
}

func expectedPassed(expected []string, actual []runnerclient.TestResult) bool {
	passed := make(map[string]bool, len(actual))
	for _, result := range actual {
		if prior, exists := passed[result.Name]; exists {
			passed[result.Name] = prior && result.Passed
		} else {
			passed[result.Name] = result.Passed
		}
	}
	for _, name := range expected {
		if !passed[name] {
			return false
		}
	}
	return true
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (h *Handler) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID()
		w.Header().Set("X-Request-ID", requestID)
		recorder := &responseRecorder{ResponseWriter: w}
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				if recorder.status == 0 {
					writeError(recorder, http.StatusInternalServerError, "internal server error")
				}
				log.Printf("request_id=%s method=%s path=%s status=%d duration=%s panic=%v", requestID, r.Method, r.URL.Path, recorder.status, time.Since(started), recovered)
				return
			}
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf("request_id=%s method=%s path=%s status=%d duration=%s", requestID, r.Method, r.URL.Path, status, time.Since(started))
		}()
		next.ServeHTTP(recorder, r)
	})
}

func requestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}
