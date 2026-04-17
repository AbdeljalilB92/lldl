// Package ui defines the Presenter interface for user interaction.
// Implementations (CLI, Wails GUI) satisfy this contract so the application
// can swap presentation layers without touching business logic.
package ui

import (
	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
)

// Presenter abstracts all user-facing output and input prompts.
// Both CLI and future Wails GUI implementations must satisfy this interface.
type Presenter interface {
	ShowError(msg string)
	ShowSuccess(msg string)
	ShowInfo(msg string)
	PromptString(label string) string
	PromptPassword(label string) string
	PromptQuality() (shareddomain.Quality, error)
	PromptYesNo(label string) bool
	PromptPath(label string) string
	ShowCourseInfo(title string, chapters int, videos int)
}
