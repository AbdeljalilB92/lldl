package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/abdeljalil/linkedin-learning-downloader/internal/model"
	"golang.org/x/term"
)

const promptGlyph = "╠══ "

// reader is the shared buffered reader used by all prompt functions.
// Using a single reader avoids data loss from mixing bufio.Reader with fmt.Scanln.
var reader = bufio.NewReader(os.Stdin)

func ShowError(msg string) {
	fmt.Printf("\033[31m%s[ERROR] %s\033[0m\n", promptGlyph, msg)
}

func ShowSuccess(msg string) {
	fmt.Printf("\033[32m%s[OK] %s\033[0m\n", promptGlyph, msg)
}

func ShowInfo(msg string) {
	fmt.Printf("%s%s\n", promptGlyph, msg)
}

func PromptString(label string) string {
	fmt.Printf("%s%s\n", promptGlyph, label)
	fmt.Print(promptGlyph)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

func PromptPassword(label string) string {
	fmt.Printf("%s%s\n", promptGlyph, label)
	fmt.Print(promptGlyph)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(pw))
}

func PromptQuality() model.Quality {
	for {
		fmt.Printf("%sWhich quality would you like?\n", promptGlyph)
		fmt.Printf("%s1) High (720p)  2) Medium (540p)  3) Low (360p)\n", promptGlyph)
		fmt.Print(promptGlyph)
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF or read error — default to high quality
			return model.QualityHigh
		}
		input := strings.TrimSpace(strings.ToLower(line))

		q, err := model.QualityFromString(input)
		if err != nil {
			ShowError("Invalid quality. Please enter 1, 2, 3, or 720p/540p/360p")
			continue
		}
		return q
	}
}

func PromptPath(label string) string {
	for {
		path := PromptString(label)
		if path == "" {
			ShowError("Path cannot be empty")
			continue
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			ShowError(fmt.Sprintf("Cannot create directory: %v", err))
			continue
		}
		return path
	}
}

func PromptYesNo(label string) bool {
	for {
		fmt.Printf("%s%s (y/n)\n", promptGlyph, label)
		fmt.Print(promptGlyph)
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF or read error — default to no
			return false
		}
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "y", "yes", "1":
			return true
		case "n", "no", "2":
			return false
		default:
			ShowError("Please enter y or n")
		}
	}
}
