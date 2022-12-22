package propagation

import (
	"io"
)

// Handler is an object responsible for processing public propagation of blobs
// with given type. It is not meant to process a particular blob.
type Handler interface {
	// Validate reads data from given reader and ensures it passes the validation
	// according to the blob type. In case of validation error, this method should
	// return an error that satisfies `errors.Is(err, ErrValidationFailed)` filter.
	Validate(hash []byte, data io.Reader) error

	// Ingest is responsible for merging the `current` dataset with an incoming `update`
	// data. The result of the merge will be written into the `result` stream.
	//
	// It is the responsibility of this method to ensure data in the `current` and `update`
	// streams contains valid data.
	Ingest(hash []byte, current, update io.Reader, result io.Writer) error
}
