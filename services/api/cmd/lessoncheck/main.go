package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kinonoja/gobot/services/api/internal/lessoncheck"
	"github.com/kinonoja/gobot/services/api/internal/lessons"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lessoncheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runnerURL := flags.String("runner-url", "http://localhost:8080", "runner service base URL")
	contentRoot := flags.String("content", "content/courses", "course content root directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	courses, err := lessons.Load(*contentRoot)
	if err != nil {
		fmt.Fprintf(stderr, "load lessons: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "LESSON\tSOLUTION\tSTARTER\tDETAIL")
	client := &http.Client{Timeout: 2 * time.Minute}
	allPassed := true
	for _, course := range courses {
		for _, chapter := range course.Chapters {
			for _, lesson := range chapter.Lessons {
				result := lessoncheck.CheckLesson(context.Background(), client, *runnerURL, course, chapter, lesson)
				solutionLabel, starterLabel := resultLabels(result)
				detail := strings.TrimSpace(strings.Join(nonEmpty(result.SolutionDetail, result.StarterDetail), "; "))
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", result.LessonPath, solutionLabel, starterLabel, detail)
				if !result.SolutionPassed || !result.StarterRejected {
					allPassed = false
				}
			}
		}
	}
	if !allPassed {
		return 1
	}
	return 0
}

func resultLabels(result lessoncheck.Result) (string, string) {
	solution := "FAIL"
	if result.SolutionPassed {
		solution = "PASS"
	}
	starter := "PASS"
	if result.StarterRejected {
		starter = "REJECTED"
	}
	return solution, starter
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
