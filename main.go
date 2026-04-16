package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/abdeljalil/linkedin-learning-downloader/internal/client"
	"github.com/abdeljalil/linkedin-learning-downloader/internal/download"
	"github.com/abdeljalil/linkedin-learning-downloader/internal/model"
	"github.com/abdeljalil/linkedin-learning-downloader/internal/tui"
)

const banner = `
╔═╗╔═╗────╔╗───╔╗──────────╔╗─────────╔╗──────────────╔╗─╔╗
║║╚╝║║────║║───║║──────────║║─────────║║──────────────║║─║║
║╔╗╔╗╠══╦═╝╠══╗║╚═╦╗─╔╦╗╔══╣╚═╦╗╔╦══╦═╝╠══╦╗─╔╦╗╔╦══╦═╣╚═╝╠══╗
║║║║║║╔╗║╔╗║║═╣║╔╗║║─║╠╝║╔╗║╔╗║╚╝║║═╣╔╗║╔╗║║─║║╚╝║╔╗║╔╬╦═╗║╔╗║
║║║║║║╔╗║╚╝║║═╣║╚╝║╚═╝╠╗║╔╗║║║║║║║║═╣╚╝║╔╗║╚═╝║║║║╔╗║║║║─║║╔╗║
╚╝╚╝╚╩╝╚╩══╩══╝╚══╩═╗╔╩╝╚╝╚╩╝╚╩╩╩╩══╩══╩╝╚╩═╗╔╩╩╩╩╝╚╩╝╚╝─╚╩╝╚╝
──────────────────╔═╝║────────────────────╔═╝║
──────────────────╚══╝────────────────────╚══╝
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Set up slog file handler
	os.MkdirAll("./logs", 0755)
	logFile, err := os.OpenFile("./logs/log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		multiWriter := io.MultiWriter(os.Stderr, logFile)
		slog.SetDefault(slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelInfo})))
	}

	// Handle Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Print(banner)

	var (
		token     string
		quality   model.Quality
		outputDir string
	)

	configPath := model.DefaultConfigPath()

	// Check for existing config
	cfg, err := model.LoadConfig(configPath)
	if err == nil && cfg != nil {
		tui.ShowInfo("Found existing config file")
		fmt.Printf("  Saved quality: %s, Saved directory: %s\n", cfg.Quality, cfg.CourseDirectory)
		if tui.PromptYesNo("Would you like to reuse the saved configuration?") {
			token = cfg.GetAuthToken()
			var qErr error
			quality, qErr = model.QualityFromString(cfg.Quality)
			if qErr != nil {
				tui.ShowInfo(fmt.Sprintf("Invalid saved quality %q, defaulting to 720p", cfg.Quality))
				quality = model.QualityHigh
			}
			outputDir = cfg.CourseDirectory
			tui.ShowSuccess("Configuration loaded")
		} else {
			cfg = nil
		}
	}

	// If no config (or user declined), prompt for values
	if cfg == nil {
		token = tui.PromptPassword("Enter your li_at token:")
		quality = tui.PromptQuality()
		outputDir = tui.PromptPath("Enter download directory path:")

		// Build new config for saving later
		cfg = &model.Config{
			CourseDirectory: outputDir,
			Quality:         quality.String(),
		}
		cfg.SetAuthToken(token)
	}

	// Prompt for course URL
	courseURL := tui.PromptString("Enter the LinkedIn Learning course URL:")
	if courseURL == "" {
		return fmt.Errorf("course URL cannot be empty")
	}

	// Create extractor
	extractor, err := client.NewExtractor(courseURL, token, quality, 1)
	if err != nil {
		return fmt.Errorf("invalid course URL: %w", err)
	}

	// Validate token
	tui.ShowInfo("Validating token...")
	if err := extractor.ValidateToken(ctx); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}
	tui.ShowSuccess("Token is valid")

	// Extract course
	tui.ShowInfo("Extracting course data. This might take some time...")
	course, err := extractor.GetCourse(ctx)
	if err != nil {
		return fmt.Errorf("failed to extract course: %w", err)
	}
	tui.ShowSuccess("Course extracted successfully")

	// Check for empty course
	if len(course.Chapters) == 0 {
		tui.ShowInfo("Warning: course has no chapters")
	}

	// Replace dead API exercise file URLs with working ambry URLs from page HTML
	if len(course.ExerciseFiles) > 0 && len(course.Chapters) > 0 && len(course.Chapters[0].Videos) > 0 {
		firstVideoSlug := course.Chapters[0].Videos[0].Slug
		ambryFiles, err := extractor.GetExerciseFileURLs(ctx, course.Slug, firstVideoSlug)
		if err != nil {
			tui.ShowInfo(fmt.Sprintf("Warning: could not fetch ambry exercise URLs: %v", err))
		} else if len(ambryFiles) > 0 {
			course.ExerciseFiles = ambryFiles
		}
	}

	// Print course info
	fmt.Println()
	fmt.Printf("  Course: %s\n", course.Title)
	fmt.Printf("  Chapters: %d\n", len(course.Chapters))
	totalVideos := 0
	for _, ch := range course.Chapters {
		totalVideos += len(ch.Videos)
	}
	fmt.Printf("  Videos: %d\n", totalVideos)
	fmt.Println()

	// Save config BEFORE download so credentials persist even if download crashes
	cfg.CourseDirectory = outputDir
	cfg.Quality = quality.String()
	cfg.SetAuthToken(token)
	if err := cfg.Save(configPath); err != nil {
		tui.ShowError(fmt.Sprintf("Failed to save config: %v", err))
	} else {
		tui.ShowSuccess("Configuration saved for next run")
	}

	// Create downloader and download course
	downloader := download.NewDownloader(outputDir, 4)
	downloader.SetAuthClient(extractor.Client(), extractor.CSRFToken())
	tui.ShowInfo("Starting download...")
	if err := downloader.DownloadCourse(ctx, course); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	tui.ShowSuccess("Download complete!")
	return nil
}
