package photos

import "errors"

// Derivative is a processed image output recorded on the photo row.
type Derivative struct {
	Kind   string
	Key    string
	Width  int
	Height int
}

// Derivatives describes the set of outputs produced by the processing worker.
type Derivatives struct {
	Thumbnail Derivative
	Medium    Derivative
	Optimized Derivative
}

// ErrNotProcessing is returned when a derivative update targets a photo that
// is no longer in the PROCESSING state.
var ErrNotProcessing = errors.New("photo is not in processing state")
