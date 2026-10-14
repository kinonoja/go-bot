package lessons

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fixturePaths struct {
	root    string
	course  string
	chapter string
	lesson  string
}

func makeFixture(t *testing.T) fixturePaths {
	t.Helper()
	root := t.TempDir()
	paths := fixturePaths{
		root:    root,
		course:  filepath.Join(root, "go-course"),
		chapter: "",
		lesson:  "",
	}
	paths.chapter = filepath.Join(paths.course, "01-first-chapter")
	paths.lesson = filepath.Join(paths.chapter, "01-first-lesson")

	writeFixtureFile(t, filepath.Join(paths.course, "course.yaml"), "title: Go Course\ndescription: Learn Go\n")
	writeFixtureFile(t, filepath.Join(paths.chapter, "chapter.yaml"), "title: First Chapter\ndescription: Start here\n")
	writeFixtureFile(t, filepath.Join(paths.lesson, "lesson.md"), fixtureLessonMarkdown())
	writeFixtureFile(t, filepath.Join(paths.lesson, "starter", "main.go"), "package main\nfunc Value() int { return 0 }\n")
	writeFixtureFile(t, filepath.Join(paths.lesson, "solution", "main.go"), "package main\nfunc Value() int { return 1 }\n")
	writeFixtureFile(t, filepath.Join(paths.lesson, "tests", "main_test.go"), "package main\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}\n")
	return paths
}

func fixtureLessonMarkdown() string {
	return "---\ntitle: First Lesson\nxp: 25\ndifficulty: easy\nexpected_tests:\n  - TestOne\n  - TestTwo\n---\n\n# Goal\n\nImplement the function.\n"
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, fixturePaths)
		check  func(*testing.T, []Course)
	}{
		{
			name: "loads content and orders numeric prefixes",
			mutate: func(t *testing.T, paths fixturePaths) {
				secondLesson := filepath.Join(paths.chapter, "02-alpha-lesson")
				writeFixtureFile(t, filepath.Join(secondLesson, "lesson.md"), fixtureLessonMarkdown())
				writeFixtureFile(t, filepath.Join(secondLesson, "starter", "main.go"), "package main\nfunc Value() int { return 0 }\n")
				writeFixtureFile(t, filepath.Join(secondLesson, "solution", "main.go"), "package main\nfunc Value() int { return 1 }\n")
				writeFixtureFile(t, filepath.Join(secondLesson, "tests", "main_test.go"), "package main\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}\n")
				secondChapter := filepath.Join(paths.course, "02-alpha-chapter")
				writeFixtureFile(t, filepath.Join(secondChapter, "chapter.yaml"), "title: Second Chapter\ndescription: Later chapter\n")
				secondChapterLesson := filepath.Join(secondChapter, "01-alpha-lesson")
				writeFixtureFile(t, filepath.Join(secondChapterLesson, "lesson.md"), fixtureLessonMarkdown())
				writeFixtureFile(t, filepath.Join(secondChapterLesson, "starter", "main.go"), "package main\nfunc Value() int { return 0 }\n")
				writeFixtureFile(t, filepath.Join(secondChapterLesson, "solution", "main.go"), "package main\nfunc Value() int { return 1 }\n")
				writeFixtureFile(t, filepath.Join(secondChapterLesson, "tests", "main_test.go"), "package main\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}\n")
			},
			check: func(t *testing.T, courses []Course) {
				if len(courses) != 1 || courses[0].Slug != "go-course" {
					t.Fatalf("courses = %#v", courses)
				}
				chapters := courses[0].Chapters
				if len(chapters) != 2 || chapters[0].Slug != "first-chapter" || chapters[1].Slug != "alpha-chapter" {
					t.Fatalf("chapter order = %#v", chapters)
				}
				if len(chapters[0].Lessons) != 2 || chapters[0].Lessons[0].Slug != "first-lesson" || chapters[0].Lessons[1].Slug != "alpha-lesson" {
					t.Fatalf("lessons = %#v", chapters[0].Lessons)
				}
			},
		},
		{
			name: "loads lesson frontmatter and file groups",
			check: func(t *testing.T, courses []Course) {
				lesson := courses[0].Chapters[0].Lessons[0]
				if lesson.Title != "First Lesson" || lesson.XP != 25 || lesson.Difficulty != "easy" || lesson.Boss {
					t.Fatalf("lesson metadata = %#v", lesson)
				}
				if !strings.Contains(lesson.Body, "# Goal") || lesson.StarterFiles["main.go"] == lesson.SolutionFilesForInternalUse()["main.go"] {
					t.Fatalf("lesson body or file data not loaded")
				}
				if len(lesson.GraderFilesForInternalUse()) != 1 {
					t.Fatalf("grader files = %#v", lesson.GraderFilesForInternalUse())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := makeFixture(t)
			if tt.mutate != nil {
				tt.mutate(t, paths)
			}
			courses, err := Load(paths.root)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			tt.check(t, courses)
		})
	}
}

func TestLoadRejectsInvalidContent(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*testing.T, fixturePaths)
		wantPath    string
		wantMessage string
	}{
		{name: "missing course metadata", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.course, "course.yaml")) }, wantPath: "course.yaml", wantMessage: "required file missing"},
		{name: "missing chapter metadata", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.chapter, "chapter.yaml")) }, wantPath: "chapter.yaml", wantMessage: "required file missing"},
		{name: "missing lesson markdown", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.lesson, "lesson.md")) }, wantPath: "lesson.md", wantMessage: "required file missing"},
		{name: "missing starter file", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.lesson, "starter", "main.go")) }, wantPath: "starter/main.go", wantMessage: "required file missing"},
		{name: "missing solution file", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.lesson, "solution", "main.go")) }, wantPath: "solution/main.go", wantMessage: "required file missing"},
		{name: "missing grader file", mutate: func(t *testing.T, p fixturePaths) { mustRemove(t, filepath.Join(p.lesson, "tests", "main_test.go")) }, wantPath: "tests/main_test.go", wantMessage: "required file missing"},
		{name: "invalid course slug", mutate: func(t *testing.T, p fixturePaths) { renameFixture(t, p.course, filepath.Join(p.root, "Go_Course")) }, wantPath: "Go_Course", wantMessage: `invalid course slug "Go_Course"`},
		{name: "invalid numeric directory format", mutate: func(t *testing.T, p fixturePaths) {
			renameFixture(t, p.chapter, filepath.Join(p.course, "1-first-chapter"))
		}, wantPath: "1-first-chapter", wantMessage: "directory must use NN-slug format"},
		{name: "invalid chapter slug", mutate: func(t *testing.T, p fixturePaths) {
			renameFixture(t, p.chapter, filepath.Join(p.course, "01-First_Chapter"))
		}, wantPath: "01-First_Chapter", wantMessage: `invalid slug "First_Chapter"`},
		{name: "chapter prefix gap", mutate: func(t *testing.T, p fixturePaths) {
			renameFixture(t, p.chapter, filepath.Join(p.course, "02-first-chapter"))
		}, wantPath: "02-first-chapter", wantMessage: "non-contiguous numeric prefix 02; expected 01"},
		{name: "duplicate chapter prefix", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.course, "01-second-chapter", "chapter.yaml"), "title: Second\ndescription: Second\n")
		}, wantPath: "01-", wantMessage: "duplicate numeric prefix 01"},
		{name: "duplicate chapter slug", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.course, "02-first-chapter", "chapter.yaml"), "title: Second\ndescription: Second\n")
		}, wantPath: "02-first-chapter", wantMessage: `duplicate chapter slug "first-chapter"`},
		{name: "lesson prefix gap", mutate: func(t *testing.T, p fixturePaths) {
			renameFixture(t, p.lesson, filepath.Join(p.chapter, "02-first-lesson"))
		}, wantPath: "02-first-lesson", wantMessage: "non-contiguous numeric prefix 02; expected 01"},
		{name: "duplicate lesson prefix", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.chapter, "01-second-lesson", "lesson.md"), fixtureLessonMarkdown())
		}, wantPath: "01-", wantMessage: "duplicate numeric prefix 01"},
		{name: "duplicate lesson slug", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.chapter, "02-first-lesson", "lesson.md"), fixtureLessonMarkdown())
		}, wantPath: "02-first-lesson", wantMessage: `duplicate lesson slug "first-lesson"`},
		{name: "missing frontmatter opening delimiter", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), "title: First Lesson\n")
		}, wantPath: "lesson.md", wantMessage: "missing YAML frontmatter opening delimiter"},
		{name: "missing frontmatter closing delimiter", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), "---\ntitle: First Lesson\n")
		}, wantPath: "lesson.md", wantMessage: "missing YAML frontmatter closing delimiter"},
		{name: "malformed frontmatter yaml", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), "---\ntitle: [\n---\nBody\n")
		}, wantPath: "lesson.md", wantMessage: "invalid YAML frontmatter"},
		{name: "unknown frontmatter field", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "xp: 25", "xp: 25\nextra: true", 1))
		}, wantPath: "lesson.md", wantMessage: "field extra not found"},
		{name: "xp below minimum", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "xp: 25", "xp: 9", 1))
		}, wantPath: "lesson.md", wantMessage: "xp must be an integer from 10 to 500"},
		{name: "xp above maximum", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "xp: 25", "xp: 501", 1))
		}, wantPath: "lesson.md", wantMessage: "xp must be an integer from 10 to 500"},
		{name: "non-integer xp", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "xp: 25", "xp: many", 1))
		}, wantPath: "lesson.md", wantMessage: "invalid YAML frontmatter"},
		{name: "empty lesson title", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "title: First Lesson", "title: ''", 1))
		}, wantPath: "lesson.md", wantMessage: "title must not be empty"},
		{name: "invalid difficulty", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "difficulty: easy", "difficulty: expert", 1))
		}, wantPath: "lesson.md", wantMessage: "difficulty must be easy, medium, or hard"},
		{name: "boss xp below minimum", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "xp: 25", "xp: 99\nboss: true", 1))
		}, wantPath: "lesson.md", wantMessage: "boss lessons must have xp of at least 100"},
		{name: "no expected tests", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "expected_tests:\n  - TestOne\n  - TestTwo\n", "expected_tests: []\n", 1))
		}, wantPath: "lesson.md", wantMessage: "at least one expected test is required"},
		{name: "invalid expected test name", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "TestOne", "Test-One", 1))
		}, wantPath: "lesson.md", wantMessage: `invalid expected test name "Test-One"`},
		{name: "duplicate expected test names", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "TestTwo", "TestOne", 1))
		}, wantPath: "lesson.md", wantMessage: `duplicate expected test name "TestOne"`},
		{name: "expected test function missing", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "lesson.md"), strings.Replace(fixtureLessonMarkdown(), "TestTwo", "TestThree", 1))
		}, wantPath: "tests/main_test.go", wantMessage: `expected test "TestThree" does not appear as a function`},
		{name: "starter identical to solution", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "starter", "main.go"), "package main\nfunc Value() int { return 1 }\n")
		}, wantPath: "starter", wantMessage: "starter files are identical to solution files"},
		{name: "starter test source rejected", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "starter", "helper_test.go"), "package main\n")
		}, wantPath: "starter/helper_test.go", wantMessage: "starter and solution files cannot end in _test.go"},
		{name: "solution test source rejected", mutate: func(t *testing.T, p fixturePaths) {
			writeFixtureFile(t, filepath.Join(p.lesson, "solution", "helper_test.go"), "package main\n")
		}, wantPath: "solution/helper_test.go", wantMessage: "starter and solution files cannot end in _test.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := makeFixture(t)
			tt.mutate(t, paths)
			_, err := Load(paths.root)
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantPath) || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("Load() error = %q, want path %q and message %q", err, tt.wantPath, tt.wantMessage)
			}
		})
	}
}

func TestPublicLessonTypeExcludesPrivateContent(t *testing.T) {
	publicType := reflect.TypeOf(PublicLesson{})
	for _, field := range []string{"SolutionFiles", "GraderFiles", "solutionFiles", "graderFiles"} {
		if _, exists := publicType.FieldByName(field); exists {
			t.Errorf("PublicLesson contains forbidden field %q", field)
		}
	}

	paths := makeFixture(t)
	courses, err := Load(paths.root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	public := courses[0].PublicView()
	lesson := public.Chapters[0].Lessons[0]
	if _, exists := lesson.StarterFiles["main.go"]; !exists || len(lesson.ExpectedTests) != 2 {
		t.Fatalf("public lesson = %#v", lesson)
	}
	if reflect.DeepEqual(lesson.StarterFiles, courses[0].Chapters[0].Lessons[0].SolutionFilesForInternalUse()) {
		t.Fatal("public starter files unexpectedly equal the private solution")
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func renameFixture(t *testing.T, oldPath, newPath string) {
	t.Helper()
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename %s: %v", oldPath, err)
	}
}
