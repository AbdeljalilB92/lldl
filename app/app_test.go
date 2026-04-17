package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/AbdeljalilB92/lldl/features/course"
	"github.com/AbdeljalilB92/lldl/features/download"
	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
)

func TestBuildDownloadJobs_EmptyCourse(t *testing.T) {
	crs := &course.Course{Slug: "empty-course"}
	jobs := buildDownloadJobs(crs, "/tmp/out")
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs for empty course, got %d", len(jobs))
	}
}

func TestBuildDownloadJobs_VideoJobs(t *testing.T) {
	crs := &course.Course{
		Title: "Go Basics",
		Slug:  "go-basics",
		Chapters: []course.Chapter{
			{
				Title:         "Introduction",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "Hello World", Slug: "hello", DownloadURL: "https://cdn.example.com/hello.mp4"},
					{Title: "Variables", Slug: "vars", DownloadURL: "https://cdn.example.com/vars.mp4"},
				},
			},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")

	// 2 video jobs.
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// First job should be a critical video download.
	if !jobs[0].Critical {
		t.Error("video job should be critical")
	}
	if jobs[0].URL != "https://cdn.example.com/hello.mp4" {
		t.Errorf("video URL = %q, want https://cdn.example.com/hello.mp4", jobs[0].URL)
	}
	expectedDest := filepath.Join("Go Basics", "01 - Introduction", "01 - Hello World.mp4")
	if !strings.HasSuffix(jobs[0].DestPath, expectedDest) {
		t.Errorf("dest path = %q, want suffix %q", jobs[0].DestPath, expectedDest)
	}
	if jobs[0].Description != "Introduction/Hello World" {
		t.Errorf("description = %q, want %q", jobs[0].Description, "Introduction/Hello World")
	}

	// Second job.
	if !jobs[1].Critical {
		t.Error("second video job should be critical")
	}
	if !strings.HasSuffix(jobs[1].DestPath, "02 - Variables.mp4") {
		t.Errorf("second dest path = %q, want suffix 02 - Variables.mp4", jobs[1].DestPath)
	}
}

func TestBuildDownloadJobs_TranscriptJob(t *testing.T) {
	crs := &course.Course{
		Slug: "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Chapter 1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{
						Title:       "Video 1",
						Slug:        "v1",
						DownloadURL: "https://cdn.example.com/v1.mp4",
						Transcript:  "1\n00:00:00,000 --> 00:00:01,000\nHello",
					},
				},
			},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")

	// 1 video job + 1 transcript job.
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs (video + transcript), got %d", len(jobs))
	}

	// Video job.
	if !jobs[0].Critical {
		t.Error("video job should be critical")
	}
	if !strings.HasSuffix(jobs[0].DestPath, ".mp4") {
		t.Errorf("video job dest should end with .mp4, got %q", jobs[0].DestPath)
	}

	// Transcript job: content-based (no URL), non-critical.
	if jobs[1].Critical {
		t.Error("transcript job should be non-critical")
	}
	if !strings.HasSuffix(jobs[1].DestPath, ".srt") {
		t.Errorf("transcript job dest should end with .srt, got %q", jobs[1].DestPath)
	}
	if string(jobs[1].Content) != "1\n00:00:00,000 --> 00:00:01,000\nHello" {
		t.Errorf("transcript content mismatch, got %q", string(jobs[1].Content))
	}
	if jobs[1].URL != "" {
		t.Errorf("transcript job should have no URL, got %q", jobs[1].URL)
	}
}

func TestBuildDownloadJobs_NoTranscriptWhenEmpty(t *testing.T) {
	crs := &course.Course{
		Slug: "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Chapter 1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{
						Title:       "Video 1",
						Slug:        "v1",
						DownloadURL: "https://cdn.example.com/v1.mp4",
						Transcript:  "",
					},
				},
			},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (video only), got %d", len(jobs))
	}
}

func TestBuildDownloadJobs_ExerciseFiles(t *testing.T) {
	crs := &course.Course{
		Title: "Go Course",
		Slug:  "go-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos:        []course.Video{},
			},
		},
		ExerciseFiles: []shareddomain.ExerciseFile{
			{FileName: "code.zip", DownloadURL: "https://cdn.example.com/code.zip"},
			{FileName: "data/readme.txt", DownloadURL: "https://cdn.example.com/readme.txt"},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")

	// 2 exercise file jobs.
	if len(jobs) != 2 {
		t.Fatalf("expected 2 exercise file jobs, got %d", len(jobs))
	}

	for _, j := range jobs {
		if j.Critical {
			t.Error("exercise file jobs should be non-critical")
		}
		expectedSubdir := filepath.Join("Go Course", "Exercise Files")
		if !strings.Contains(j.DestPath, expectedSubdir) {
			t.Errorf("exercise dest = %q, want to contain %q", j.DestPath, expectedSubdir)
		}
	}

	// Filenames should be sanitized (no slashes in the base name).
	if strings.Contains(filepath.Base(jobs[1].DestPath), "/") {
		t.Errorf("exercise filename should be sanitized, got %q", filepath.Base(jobs[1].DestPath))
	}
}

func TestBuildDownloadJobs_MultipleChapters(t *testing.T) {
	crs := &course.Course{
		Slug: "multi-ch",
		Chapters: []course.Chapter{
			{
				Title:         "First Chapter",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "V1", DownloadURL: "https://cdn.example.com/v1.mp4"},
				},
			},
			{
				Title:         "Second Chapter",
				IndexInCourse: 1,
				Videos: []course.Video{
					{Title: "V2", DownloadURL: "https://cdn.example.com/v2.mp4"},
					{Title: "V3", DownloadURL: "https://cdn.example.com/v3.mp4"},
				},
			},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")

	// 3 video jobs total.
	if len(jobs) != 3 {
		t.Fatalf("expected 3 video jobs, got %d", len(jobs))
	}

	// Verify chapter directory numbering.
	if !strings.Contains(jobs[0].DestPath, "01 - First Chapter") {
		t.Errorf("first chapter dir = %q, want to contain '01 - First Chapter'", jobs[0].DestPath)
	}
	if !strings.Contains(jobs[1].DestPath, "02 - Second Chapter") {
		t.Errorf("second chapter dir = %q, want to contain '02 - Second Chapter'", jobs[1].DestPath)
	}
}

func TestBuildDownloadJobs_JobTypes(t *testing.T) {
	// Verify that video jobs have URL set, content jobs have Content set
	// (no URL), and exercise jobs have URL set.
	crs := &course.Course{
		Slug: "test",
		Chapters: []course.Chapter{
			{
				Title:         "Ch",
				IndexInCourse: 0,
				Videos: []course.Video{
					{
						Title:       "WithTranscript",
						DownloadURL: "https://cdn.example.com/vid.mp4",
						Transcript:  "SRT content",
					},
				},
			},
		},
		ExerciseFiles: []shareddomain.ExerciseFile{
			{FileName: "ex.zip", DownloadURL: "https://cdn.example.com/ex.zip"},
		},
	}

	jobs := buildDownloadJobs(crs, "/tmp/out")

	// Job 0: video download (URL-based, critical).
	assertURLJob(t, jobs[0], true)
	// Job 1: transcript write (content-based, non-critical).
	assertContentJob(t, jobs[1], false)
	// Job 2: exercise file (URL-based, non-critical).
	assertURLJob(t, jobs[2], false)
}

func assertURLJob(t *testing.T, job download.Job, wantCritical bool) {
	t.Helper()
	if job.URL == "" {
		t.Error("URL job should have non-empty URL")
	}
	if len(job.Content) != 0 {
		t.Errorf("URL job should have empty Content, got %q", string(job.Content))
	}
	if job.Critical != wantCritical {
		t.Errorf("job Critical = %v, want %v", job.Critical, wantCritical)
	}
}

func assertContentJob(t *testing.T, job download.Job, wantCritical bool) {
	t.Helper()
	if len(job.Content) == 0 {
		t.Error("content job should have non-empty Content")
	}
	if job.URL != "" {
		t.Errorf("content job should have empty URL, got %q", job.URL)
	}
	if job.Critical != wantCritical {
		t.Errorf("job Critical = %v, want %v", job.Critical, wantCritical)
	}
}
