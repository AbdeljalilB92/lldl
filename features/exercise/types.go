// Package exercise defines the interface and logic for resolving exercise file
// download URLs from LinkedIn Learning course pages. It imports only from
// shared/ and lib/ — no cross-feature imports.
package exercise

import shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"

// Result holds the exercise files extracted from a course page.
type Result struct {
	Files []shareddomain.ExerciseFile
}
