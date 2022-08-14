package datastore

import (
	"errors"
	"io"
)

var (
	// ErrNotFound will be used when blob with given name was not found in datastore
	ErrNotFound = errors.New("blob not found")

	// ErrInvalidData indicates that the data retrieved in the Update call
	ErrInvalidData = errors.New("invalid data")
)

// DS interface contains the public interface of any conformant datastore
//
// Stored data is split into small chunks called blobs. Each blob has its
// unique name. The name is used to perform cryptographic verification
// of the blob data (e.g. it must match the digest of data for static blobs).
//
// The blob content (in case of blobs other than the static blob) can be updated
// over time. The rule of forward progress is that if there are two or more
// valid datasets for a single blob, performing update of that blob
// with those datasets must be deterministic and always result with a single
// final dataset. This may be one of the source datasets or a combined dataset
// containing information from other datasets merged.
//
// On the interface level, there is no distinction between blob types and
// their internal data. Working with that interface allows treating the dataset
// as completely opaque byte streams. This simplifies implementation of
// data various transfer mechanisms independently from blob types.
type DS interface {

	// Kind returns string representation of datastore kind (i.e. "Memory")
	Kind() string

	// Read returns a read stream for given blob name or an error. In case blob
	// is not found in datastore, returned error must be ErrNotFound.
	// In case of returning a stream, caller must ensure to call Close on it
	// after reading it's contents. This function must guarantee that the
	// returned contents does pass the validation of blob data.
	// If it does not, ErrInvalidData must be returned instead of io.EOF.
	// This check is needed to ensure the underlying data has not been
	// tempered with (chosen ciphertext attack)
	Read(name BlobName, output io.Writer) error

	// Update retrieves an update for given blob. The data is read from given
	// reader until it returns either EOF, ending successful save, or any other
	// error which will cancel the save - in such case this error will be
	// returned from this function. If the data does not pass validation,
	// ErrInvalidData will be returned.
	Update(name BlobName, r io.Reader) error

	// Exists does check whether blob of given name exists in the datastore.
	// Partially written blobs are equal to non-existing ones. Boolean value
	// returned indicates whether the blob exists or not, non-nil error indicates
	// that there was an error while trying to check blob's existence.
	Exists(name BlobName) (bool, error)

	// Delete tries to remove blob with given name from the datastore.
	// If blob does not exist (which includes partially written blobs)
	// ErrNotFound will be returned. If blob is being opened at the moment
	// of removal, all opened references to the blob must still be able to
	// read the blob data. After the `Delete` call succeeds, trying to read
	// the blob with the `Open` should end up with an ErrNotFound error
	// until the blob is updated again with a successful `Update` call.
	Delete(name BlobName) error
}
