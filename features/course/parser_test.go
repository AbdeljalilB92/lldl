package course

import (
	"errors"
	"strings"
	"testing"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharederr "github.com/AbdeljalilB92/lldl/shared/errors"
)

func TestParseCourse_ValidResponse(t *testing.T) {
	body := []byte(`{
		"elements": [{
			"title": "Go Fundamentals",
			"slug": "go-fundamentals",
			"chapters": [
				{
					"title": "Getting Started",
					"slug": "getting-started",
					"videos": [
						{"title": "Welcome", "slug": "welcome"},
						{"title": "Setup", "slug": "setup"}
					]
				},
				{
					"title": "Core Concepts",
					"slug": "core-concepts",
					"videos": [
						{"title": "Types", "slug": "types"}
					]
				}
			],
			"exerciseFiles": [
				{"name": "exercises.zip", "url": "https://example.com/ex.zip", "sizeInBytes": 1024}
			]
		}],
		"paging": {"start": 0, "count": 1, "total": 1}
	}`)

	course, err := ParseCourse(body)
	if err != nil {
		t.Fatalf("ParseCourse returned error: %v", err)
	}

	if course.Title != "Go Fundamentals" {
		t.Errorf("Title = %q, want %q", course.Title, "Go Fundamentals")
	}
	if course.Slug != "go-fundamentals" {
		t.Errorf("Slug = %q, want %q", course.Slug, "go-fundamentals")
	}

	// Chapters
	if len(course.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2", len(course.Chapters))
	}

	ch0 := course.Chapters[0]
	if ch0.Title != "Getting Started" {
		t.Errorf("Chapters[0].Title = %q, want %q", ch0.Title, "Getting Started")
	}
	if ch0.IndexInCourse != 0 {
		t.Errorf("Chapters[0].IndexInCourse = %d, want 0", ch0.IndexInCourse)
	}
	if len(ch0.Videos) != 2 {
		t.Fatalf("len(Chapters[0].Videos) = %d, want 2", len(ch0.Videos))
	}
	if ch0.Videos[0].Title != "Welcome" {
		t.Errorf("Chapters[0].Videos[0].Title = %q, want %q", ch0.Videos[0].Title, "Welcome")
	}
	if ch0.Videos[1].Slug != "setup" {
		t.Errorf("Chapters[0].Videos[1].Slug = %q, want %q", ch0.Videos[1].Slug, "setup")
	}

	ch1 := course.Chapters[1]
	if ch1.Title != "Core Concepts" {
		t.Errorf("Chapters[1].Title = %q, want %q", ch1.Title, "Core Concepts")
	}
	if ch1.IndexInCourse != 1 {
		t.Errorf("Chapters[1].IndexInCourse = %d, want 1", ch1.IndexInCourse)
	}
	if len(ch1.Videos) != 1 {
		t.Fatalf("len(Chapters[1].Videos) = %d, want 1", len(ch1.Videos))
	}

	// Exercise files
	if len(course.ExerciseFiles) != 1 {
		t.Fatalf("len(ExerciseFiles) = %d, want 1", len(course.ExerciseFiles))
	}
	ef := course.ExerciseFiles[0]
	if ef.FileName != "exercises.zip" {
		t.Errorf("ExerciseFiles[0].FileName = %q, want %q", ef.FileName, "exercises.zip")
	}
	if ef.FileSize != 1024 {
		t.Errorf("ExerciseFiles[0].FileSize = %d, want 1024", ef.FileSize)
	}
}

func TestParseCourse_EmptyElements(t *testing.T) {
	body := []byte(`{"elements": [], "paging": {"start": 0, "count": 0, "total": 0}}`)

	_, err := ParseCourse(body)
	if err == nil {
		t.Fatal("ParseCourse should return error for empty elements")
	}

	// Verify it's a ParseError
	var parseErr *sharederr.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("error type = %T, want *ParseError", err)
	}
}

func TestParseCourse_EmptyChapters(t *testing.T) {
	body := []byte(`{
		"elements": [{
			"title": "Empty Course",
			"slug": "empty-course",
			"chapters": [],
			"exerciseFiles": []
		}],
		"paging": {"start": 0, "count": 1, "total": 1}
	}`)

	course, err := ParseCourse(body)
	if err != nil {
		t.Fatalf("ParseCourse returned error: %v", err)
	}
	if course.Title != "Empty Course" {
		t.Errorf("Title = %q, want %q", course.Title, "Empty Course")
	}
	if course.Chapters != nil {
		t.Errorf("Chapters = %v, want nil", course.Chapters)
	}
	if course.ExerciseFiles != nil {
		t.Errorf("ExerciseFiles = %v, want nil", course.ExerciseFiles)
	}
}

func TestParseCourse_InvalidJSON(t *testing.T) {
	body := []byte(`not json at all`)

	_, err := ParseCourse(body)
	if err == nil {
		t.Fatal("ParseCourse should return error for invalid JSON")
	}
}

func TestParseChapters_NilElement(t *testing.T) {
	el := &courseElement{}
	chapters := ParseChapters(el)
	if chapters != nil {
		t.Errorf("expected nil for empty chapters, got %v", chapters)
	}
}

func TestParseExerciseFiles_NilElement(t *testing.T) {
	el := &courseElement{}
	files := ParseExerciseFiles(el)
	if files != nil {
		t.Errorf("expected nil for empty exercise files, got %v", files)
	}
}

func TestVideo_FormTranscript(t *testing.T) {
	video := &Video{
		Duration: 120,
		TranscriptLines: []shareddomain.TranscriptLine{
			{Caption: "Hello world", StartsAt: 0},
			{Caption: "Second line", StartsAt: 3000},
			{Caption: "Last line", StartsAt: 6000},
		},
	}
	video.FormTranscript()

	if video.Transcript == "" {
		t.Fatal("FormTranscript produced empty transcript")
	}

	// First entry should have sequence 1 and end at 3000ms (next line's start).
	if !contains(video.Transcript, "1\n00:00:00,000 --> 00:00:03,000\nHello world") {
		t.Errorf("transcript missing first entry, got:\n%s", video.Transcript)
	}

	// Last entry should fall back to video duration (120s = 120000ms).
	if !contains(video.Transcript, "3\n00:00:06,000 --> 00:02:00,000\nLast line") {
		t.Errorf("transcript missing correct last entry end time, got:\n%s", video.Transcript)
	}
}

func TestVideo_FormTranscript_EmptyLines(t *testing.T) {
	video := &Video{Duration: 10}
	video.FormTranscript()
	if video.Transcript != "" {
		t.Errorf("expected empty transcript, got %q", video.Transcript)
	}
}

func TestFormatSRTTime(t *testing.T) {
	tests := []struct {
		ms     int64
		expect string
	}{
		{0, "00:00:00,000"},
		{1000, "00:00:01,000"},
		{61000, "00:01:01,000"},
		{3661500, "01:01:01,500"},
	}
	for _, tt := range tests {
		got := shareddomain.FormatSRTTime(tt.ms)
		if got != tt.expect {
			t.Errorf("formatSRTTime(%d) = %q, want %q", tt.ms, got, tt.expect)
		}
	}
}

func TestBuildCourseAPIURL(t *testing.T) {
	u := buildCourseAPIURL("my-course-slug")
	if !contains(u, "courseSlug=my-course-slug") {
		t.Errorf("URL missing courseSlug param, got: %s", u)
	}
	if !contains(u, "fields=chapters%2Ctitle%2CexerciseFiles") {
		t.Errorf("URL missing fields param, got: %s", u)
	}
	if !contains(u, "q=slugs") {
		t.Errorf("URL missing q param, got: %s", u)
	}
	if !contains(u, "addParagraphsToTranscript=true") {
		t.Errorf("URL missing addParagraphsToTranscript param, got: %s", u)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
