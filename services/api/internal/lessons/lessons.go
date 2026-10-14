package lessons

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var expectedTestPattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

type Course struct {
	Slug        string
	Title       string
	Description string
	Chapters    []Chapter
}

type Chapter struct {
	Slug        string
	Order       int
	Title       string
	Description string
	Lessons     []Lesson
}

type Lesson struct {
	Slug          string
	Order         int
	Title         string
	XP            int
	Difficulty    string
	Boss          bool
	Body          string
	StarterFiles  map[string]string
	ExpectedTests []string
	solutionFiles map[string]string
	graderFiles   map[string]string
}

type PublicCourse struct {
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Chapters    []PublicChapter `json:"chapters"`
}

type PublicChapter struct {
	Slug        string         `json:"slug"`
	Order       int            `json:"order"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Lessons     []PublicLesson `json:"lessons"`
}

type PublicLesson struct {
	Slug          string            `json:"slug"`
	Order         int               `json:"order"`
	Title         string            `json:"title"`
	XP            int               `json:"xp"`
	Difficulty    string            `json:"difficulty"`
	Boss          bool              `json:"boss"`
	Body          string            `json:"body"`
	StarterFiles  map[string]string `json:"starter_files"`
	ExpectedTests []string          `json:"expected_tests"`
}

type courseMetadata struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type lessonMetadata struct {
	Title         string   `yaml:"title"`
	XP            int      `yaml:"xp"`
	Difficulty    string   `yaml:"difficulty"`
	Boss          bool     `yaml:"boss"`
	ExpectedTests []string `yaml:"expected_tests"`
}

type orderedDirectory struct {
	name  string
	slug  string
	order int
	path  string
}

func Load(root string) ([]Course, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("%s: read course root: %w", root, err)
	}

	var courses []Course
	seenSlugs := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		coursePath := filepath.Join(root, entry.Name())
		if !slugPattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("%s: invalid course slug %q", coursePath, entry.Name())
		}
		if prior, exists := seenSlugs[entry.Name()]; exists {
			return nil, fmt.Errorf("%s: duplicate course slug %q also found at %s", coursePath, entry.Name(), prior)
		}
		seenSlugs[entry.Name()] = coursePath

		course, err := loadCourse(coursePath, entry.Name())
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	if len(courses) == 0 {
		return nil, fmt.Errorf("%s: no course directories found", root)
	}
	sort.Slice(courses, func(i, j int) bool { return courses[i].Slug < courses[j].Slug })
	return courses, nil
}

func loadCourse(path, slug string) (Course, error) {
	var metadata courseMetadata
	if err := readYAML(filepath.Join(path, "course.yaml"), &metadata); err != nil {
		return Course{}, err
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return Course{}, fmt.Errorf("%s: title must not be empty", filepath.Join(path, "course.yaml"))
	}
	if strings.TrimSpace(metadata.Description) == "" {
		return Course{}, fmt.Errorf("%s: description must not be empty", filepath.Join(path, "course.yaml"))
	}

	directories, err := orderedDirectories(path)
	if err != nil {
		return Course{}, err
	}
	chapters := make([]Chapter, 0, len(directories))
	seenSlugs := make(map[string]string)
	for _, directory := range directories {
		if prior, exists := seenSlugs[directory.slug]; exists {
			return Course{}, fmt.Errorf("%s: duplicate chapter slug %q also found at %s", directory.path, directory.slug, prior)
		}
		seenSlugs[directory.slug] = directory.path
		chapter, err := loadChapter(directory)
		if err != nil {
			return Course{}, err
		}
		chapters = append(chapters, chapter)
	}
	return Course{
		Slug:        slug,
		Title:       metadata.Title,
		Description: metadata.Description,
		Chapters:    chapters,
	}, nil
}

func loadChapter(directory orderedDirectory) (Chapter, error) {
	metadataPath := filepath.Join(directory.path, "chapter.yaml")
	var metadata courseMetadata
	if err := readYAML(metadataPath, &metadata); err != nil {
		return Chapter{}, err
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return Chapter{}, fmt.Errorf("%s: title must not be empty", metadataPath)
	}
	if strings.TrimSpace(metadata.Description) == "" {
		return Chapter{}, fmt.Errorf("%s: description must not be empty", metadataPath)
	}

	lessonDirectories, err := orderedDirectories(directory.path)
	if err != nil {
		return Chapter{}, err
	}
	lessons := make([]Lesson, 0, len(lessonDirectories))
	seenSlugs := make(map[string]string)
	for _, lessonDirectory := range lessonDirectories {
		if prior, exists := seenSlugs[lessonDirectory.slug]; exists {
			return Chapter{}, fmt.Errorf("%s: duplicate lesson slug %q also found at %s", lessonDirectory.path, lessonDirectory.slug, prior)
		}
		seenSlugs[lessonDirectory.slug] = lessonDirectory.path
		lesson, err := loadLesson(lessonDirectory)
		if err != nil {
			return Chapter{}, err
		}
		lessons = append(lessons, lesson)
	}
	return Chapter{
		Slug:        directory.slug,
		Order:       directory.order,
		Title:       metadata.Title,
		Description: metadata.Description,
		Lessons:     lessons,
	}, nil
}

func loadLesson(directory orderedDirectory) (Lesson, error) {
	lessonPath := filepath.Join(directory.path, "lesson.md")
	content, err := readRequiredFile(lessonPath)
	if err != nil {
		return Lesson{}, err
	}
	metadata, body, err := parseLessonMarkdown(lessonPath, content)
	if err != nil {
		return Lesson{}, err
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return Lesson{}, fmt.Errorf("%s: title must not be empty", lessonPath)
	}
	if metadata.XP < 10 || metadata.XP > 500 {
		return Lesson{}, fmt.Errorf("%s: xp must be an integer from 10 to 500", lessonPath)
	}
	if metadata.Difficulty != "easy" && metadata.Difficulty != "medium" && metadata.Difficulty != "hard" {
		return Lesson{}, fmt.Errorf("%s: difficulty must be easy, medium, or hard", lessonPath)
	}
	if metadata.Boss && metadata.XP < 100 {
		return Lesson{}, fmt.Errorf("%s: boss lessons must have xp of at least 100", lessonPath)
	}
	if len(metadata.ExpectedTests) == 0 {
		return Lesson{}, fmt.Errorf("%s: at least one expected test is required", lessonPath)
	}

	seenTests := make(map[string]struct{}, len(metadata.ExpectedTests))
	for _, testName := range metadata.ExpectedTests {
		if !expectedTestPattern.MatchString(testName) {
			return Lesson{}, fmt.Errorf("%s: invalid expected test name %q", lessonPath, testName)
		}
		if _, exists := seenTests[testName]; exists {
			return Lesson{}, fmt.Errorf("%s: duplicate expected test name %q", lessonPath, testName)
		}
		seenTests[testName] = struct{}{}
	}

	starterFiles, err := readGoFiles(filepath.Join(directory.path, "starter"), "main.go", false)
	if err != nil {
		return Lesson{}, err
	}
	solutionFiles, err := readGoFiles(filepath.Join(directory.path, "solution"), "main.go", false)
	if err != nil {
		return Lesson{}, err
	}
	graderFiles, err := readGoFiles(filepath.Join(directory.path, "tests"), "main_test.go", true)
	if err != nil {
		return Lesson{}, err
	}
	if equalFiles(starterFiles, solutionFiles) {
		return Lesson{}, fmt.Errorf("%s: starter files are identical to solution files", filepath.Join(directory.path, "starter"))
	}

	testPath := filepath.Join(directory.path, "tests", "main_test.go")
	testSource := graderFiles["main_test.go"]
	if err := validateExpectedTestFunctions(testPath, testSource, metadata.ExpectedTests); err != nil {
		return Lesson{}, err
	}

	return Lesson{
		Slug:          directory.slug,
		Order:         directory.order,
		Title:         metadata.Title,
		XP:            metadata.XP,
		Difficulty:    metadata.Difficulty,
		Boss:          metadata.Boss,
		Body:          body,
		StarterFiles:  starterFiles,
		ExpectedTests: append([]string(nil), metadata.ExpectedTests...),
		solutionFiles: solutionFiles,
		graderFiles:   graderFiles,
	}, nil
}

func (course Course) PublicView() PublicCourse {
	view := PublicCourse{
		Slug:        course.Slug,
		Title:       course.Title,
		Description: course.Description,
		Chapters:    make([]PublicChapter, 0, len(course.Chapters)),
	}
	for _, chapter := range course.Chapters {
		chapterView := PublicChapter{
			Slug:        chapter.Slug,
			Order:       chapter.Order,
			Title:       chapter.Title,
			Description: chapter.Description,
			Lessons:     make([]PublicLesson, 0, len(chapter.Lessons)),
		}
		for _, lesson := range chapter.Lessons {
			chapterView.Lessons = append(chapterView.Lessons, lesson.PublicView())
		}
		view.Chapters = append(view.Chapters, chapterView)
	}
	return view
}

func (lesson Lesson) PublicView() PublicLesson {
	return PublicLesson{
		Slug:          lesson.Slug,
		Order:         lesson.Order,
		Title:         lesson.Title,
		XP:            lesson.XP,
		Difficulty:    lesson.Difficulty,
		Boss:          lesson.Boss,
		Body:          lesson.Body,
		StarterFiles:  cloneFiles(lesson.StarterFiles),
		ExpectedTests: append([]string(nil), lesson.ExpectedTests...),
	}
}

func (lesson Lesson) SolutionFilesForInternalUse() map[string]string {
	return cloneFiles(lesson.solutionFiles)
}

func (lesson Lesson) GraderFilesForInternalUse() map[string]string {
	return cloneFiles(lesson.graderFiles)
}

func orderedDirectories(parent string) ([]orderedDirectory, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("%s: read directory: %w", parent, err)
	}

	directories := make([]orderedDirectory, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(parent, entry.Name())
		order, slug, err := parseOrderedSlug(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		directories = append(directories, orderedDirectory{name: entry.Name(), slug: slug, order: order, path: path})
	}
	sort.Slice(directories, func(i, j int) bool { return directories[i].order < directories[j].order })

	seenOrders := make(map[int]string, len(directories))
	for _, directory := range directories {
		if prior, exists := seenOrders[directory.order]; exists {
			return nil, fmt.Errorf("%s: duplicate numeric prefix %02d also found at %s", directory.path, directory.order, prior)
		}
		seenOrders[directory.order] = directory.path
	}
	for index, directory := range directories {
		want := index + 1
		if directory.order != want {
			return nil, fmt.Errorf("%s: non-contiguous numeric prefix %02d; expected %02d", directory.path, directory.order, want)
		}
	}
	return directories, nil
}

func parseOrderedSlug(name string) (int, string, error) {
	if len(name) < 4 || name[2] != '-' {
		return 0, "", fmt.Errorf("directory must use NN-slug format")
	}
	if name[0] < '0' || name[0] > '9' || name[1] < '0' || name[1] > '9' {
		return 0, "", fmt.Errorf("directory must use NN-slug format")
	}
	order, err := strconv.Atoi(name[:2])
	if err != nil || order == 0 {
		return 0, "", fmt.Errorf("numeric prefix must be from 01 to 99")
	}
	slug := name[3:]
	if !slugPattern.MatchString(slug) {
		return 0, "", fmt.Errorf("invalid slug %q", slug)
	}
	return order, slug, nil
}

func readYAML(path string, target any) error {
	data, err := readRequiredFile(path)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: invalid YAML: %w", path, err)
	}
	return nil
}

func parseLessonMarkdown(path string, content []byte) (lessonMetadata, string, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSuffix(lines[0], "\r") != "---" {
		return lessonMetadata{}, "", fmt.Errorf("%s: missing YAML frontmatter opening delimiter", path)
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSuffix(lines[index], "\r") == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return lessonMetadata{}, "", fmt.Errorf("%s: missing YAML frontmatter closing delimiter", path)
	}

	frontmatter := strings.Join(lines[1:closing], "\n")
	var metadata lessonMetadata
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return lessonMetadata{}, "", fmt.Errorf("%s: invalid YAML frontmatter: %w", path, err)
	}
	body := strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	return metadata, body, nil
}

func readGoFiles(directory, requiredName string, testFiles bool) (map[string]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("%s: read source directory: %w", directory, err)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if entry.IsDir() {
			return nil, fmt.Errorf("%s: nested directories are not supported", path)
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil, fmt.Errorf("%s: source file must end in .go", path)
		}
		if testFiles && !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil, fmt.Errorf("%s: grader file must end in _test.go", path)
		}
		if !testFiles && strings.HasSuffix(entry.Name(), "_test.go") {
			return nil, fmt.Errorf("%s: starter and solution files cannot end in _test.go", path)
		}
		data, err := readRequiredFile(path)
		if err != nil {
			return nil, err
		}
		files[entry.Name()] = string(data)
	}
	if _, exists := files[requiredName]; !exists {
		return nil, fmt.Errorf("%s: required file missing", filepath.Join(directory, requiredName))
	}
	return files, nil
}

func readRequiredFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: required file missing", path)
		}
		return nil, fmt.Errorf("%s: stat file: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: expected a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: read file: %w", path, err)
	}
	return data, nil
}

func validateExpectedTestFunctions(path, source string, expected []string) error {
	file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("%s: parse grader tests: %w", path, err)
	}
	functions := make(map[string]struct{})
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil {
			functions[function.Name.Name] = struct{}{}
		}
	}
	for _, name := range expected {
		if _, exists := functions[name]; !exists {
			return fmt.Errorf("%s: expected test %q does not appear as a function", path, name)
		}
	}
	return nil
}

func equalFiles(first, second map[string]string) bool {
	if len(first) != len(second) {
		return false
	}
	for name, content := range first {
		if second[name] != content {
			return false
		}
	}
	return true
}

func cloneFiles(files map[string]string) map[string]string {
	copyOfFiles := make(map[string]string, len(files))
	for name, content := range files {
		copyOfFiles[name] = content
	}
	return copyOfFiles
}
