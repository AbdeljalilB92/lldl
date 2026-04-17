package course

import (
	"encoding/json"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharederr "github.com/AbdeljalilB92/lldl/shared/errors"
)

// ParseCourse decodes a raw LinkedIn detailedCourses API JSON body into a typed
// Course struct. It extracts the first element from the response and delegates
// chapter and exercise file parsing.
func ParseCourse(body []byte) (*Course, error) {
	var resp coursesAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &sharederr.ParseError{Source: "course API response JSON", Cause: err}
	}

	if len(resp.Elements) == 0 {
		return nil, &sharederr.ParseError{
			Source: "course API response",
			Cause:  jsonErr("elements array is empty"),
		}
	}

	el := resp.Elements[0]
	return &Course{
		Title:         el.Title,
		Slug:          el.Slug,
		Chapters:      ParseChapters(&el),
		ExerciseFiles: ParseExerciseFiles(&el),
	}, nil
}

// ParseChapters converts the API chapter references into typed Chapter structs
// with their nested Video entries.
func ParseChapters(el *courseElement) []Chapter {
	if len(el.Chapters) == 0 {
		return nil
	}

	chapters := make([]Chapter, 0, len(el.Chapters))
	for i, ch := range el.Chapters {
		videos := make([]Video, 0, len(ch.Videos))
		for _, vr := range ch.Videos {
			videos = append(videos, Video{
				Title: vr.Title,
				Slug:  vr.Slug,
			})
		}
		chapters = append(chapters, Chapter{
			Title:         ch.Title,
			Slug:          ch.Slug,
			Videos:        videos,
			IndexInCourse: i,
		})
	}
	return chapters
}

// ParseExerciseFiles converts API exercise file references into shared domain types.
func ParseExerciseFiles(el *courseElement) []shareddomain.ExerciseFile {
	if len(el.ExerciseFiles) == 0 {
		return nil
	}

	files := make([]shareddomain.ExerciseFile, 0, len(el.ExerciseFiles))
	for _, ef := range el.ExerciseFiles {
		files = append(files, ef.toExerciseFile())
	}
	return files
}

// jsonErr creates a sentinel error for parse failures with a descriptive message.
// Avoids importing fmt in the hot path.
func jsonErr(msg string) error {
	return &sharederr.ParseError{Source: "course API response", Cause: parseErr(msg)}
}

type parseErr string

func (e parseErr) Error() string { return string(e) }
