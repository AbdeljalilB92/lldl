package exercise

// bprData represents the top-level JSON object embedded in a BPR <code> block
// on the LinkedIn Learning course page.
type bprData struct {
	Included []bprIncludedItem `json:"included"`
}

// bprIncludedItem is a single entry in the "included" array of the BPR payload.
// Only items with the Course type are relevant for exercise files.
type bprIncludedItem struct {
	DollarType    string          `json:"$type"`
	ExerciseFiles []exerciseEntry `json:"exerciseFiles"`
}

// exerciseEntry carries the metadata and download URL for a single exercise file.
// The URL field contains a pre-signed ambry CDN link that works immediately.
type exerciseEntry struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	SizeInBytes int64  `json:"sizeInBytes"`
}
