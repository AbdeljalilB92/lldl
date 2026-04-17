package course

import shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"

// coursesAPIResponse is the top-level shape of the LinkedIn detailedCourses API JSON.
type coursesAPIResponse struct {
	Elements []courseElement `json:"elements"`
	Paging   pagingInfo      `json:"paging"`
}

// courseElement represents a single course within the API response elements array.
type courseElement struct {
	Title         string            `json:"title"`
	Slug          string            `json:"slug"`
	Chapters      []chapterRef      `json:"chapters"`
	ExerciseFiles []exerciseFileRef `json:"exerciseFiles"`
}

// chapterRef represents a chapter entry in the course's chapter list.
type chapterRef struct {
	Title  string     `json:"title"`
	Slug   string     `json:"slug"`
	Videos []videoRef `json:"videos"`
}

// videoRef represents a video entry within a chapter.
type videoRef struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

// exerciseFileRef represents an exercise file attached to a course.
type exerciseFileRef struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	SizeInBytes int64  `json:"sizeInBytes"`
}

// pagingInfo captures pagination metadata from the API response.
type pagingInfo struct {
	Start int `json:"start"`
	Count int `json:"count"`
	Total int `json:"total"`
}

// toExerciseFile converts an API exercise file reference into a shared domain type.
func (ef exerciseFileRef) toExerciseFile() shareddomain.ExerciseFile {
	return shareddomain.ExerciseFile{
		FileName:    ef.Name,
		DownloadURL: ef.URL,
		FileSize:    ef.SizeInBytes,
	}
}
