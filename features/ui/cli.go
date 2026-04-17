package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	"golang.org/x/term"
)

const promptGlyph = "╠══ "

// cliPresenter implements Presenter using terminal I/O.
type cliPresenter struct {
	reader *bufio.Reader
}

// Compile-time guarantee that cliPresenter satisfies the Presenter interface.
var _ Presenter = (*cliPresenter)(nil)

// NewCLI creates a Presenter backed by standard terminal I/O.
func NewCLI() Presenter {
	return &cliPresenter{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (c *cliPresenter) ShowError(msg string) {
	fmt.Printf("\033[31m%s[ERROR] %s\033[0m\n", promptGlyph, msg)
}

func (c *cliPresenter) ShowSuccess(msg string) {
	fmt.Printf("\033[32m%s[OK] %s\033[0m\n", promptGlyph, msg)
}

func (c *cliPresenter) ShowInfo(msg string) {
	fmt.Printf("%s%s\n", promptGlyph, msg)
}

func (c *cliPresenter) PromptString(label string) string {
	fmt.Printf("%s%s\n", promptGlyph, label)
	fmt.Print(promptGlyph)
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

func (c *cliPresenter) PromptPassword(label string) string {
	fmt.Printf("%s%s\n", promptGlyph, label)
	fmt.Print(promptGlyph)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(pw))
}

// PromptQuality repeatedly prompts until the user enters a valid quality value.
// Valid inputs: 1/2/3, high/medium/low, or 720p/540p/360p.
func (c *cliPresenter) PromptQuality() (shareddomain.Quality, error) {
	for {
		c.ShowInfo("Which quality would you like?")
		c.ShowInfo("1) High (720p)  2) Medium (540p)  3) Low (360p)")
		fmt.Print(promptGlyph)
		line, readErr := c.reader.ReadString('\n')
		if readErr != nil {
			// EOF or read error — default to high quality
			return shareddomain.QualityHigh, readErr
		}
		input := strings.TrimSpace(strings.ToLower(line))

		q, err := shareddomain.QualityFromString(input)
		if err != nil {
			c.ShowError("Invalid quality. Please enter 1, 2, 3, or 720p/540p/360p")
			continue
		}
		return q, nil
	}
}

func (c *cliPresenter) PromptYesNo(label string) bool {
	for {
		fmt.Printf("%s%s (y/n)\n", promptGlyph, label)
		fmt.Print(promptGlyph)
		line, err := c.reader.ReadString('\n')
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
			c.ShowError("Please enter y or n")
		}
	}
}

// PromptPath repeatedly prompts until the user provides a valid, non-empty path.
// Directory creation is deferred to download time — this function only validates
// that the path is usable (non-empty, no invalid characters).
func (c *cliPresenter) PromptPath(label string) string {
	for {
		path := c.PromptString(label)
		if path == "" {
			c.ShowError("Path cannot be empty")
			continue
		}
		return path
	}
}

func (c *cliPresenter) ShowCourseInfo(title string, chapters int, videos int) {
	c.ShowSuccess(fmt.Sprintf("Course: %s", title))
	c.ShowInfo(fmt.Sprintf("Chapters: %d | Videos: %d", chapters, videos))
}
