package cas

import (
	"errors"
	"io"
)

var (
	// ErrNotFound will be used when blob with given name was not found in CAS
	ErrNotFound = errors.New("Data not found")
	// ErrNameMismatch will be used to indicate that blob's name does not match
	// blob's contents
	ErrNameMismatch = errors.New("Data name mismatch")
)

// CAS interface contains the public interface of any conformant CAS storage
type CAS interface {

	// Kind returns string representation of CAS kind (i.e. "Memory")
	Kind() string

	// Open returns a read stream for given blob name or an error. In case blob
	// is not found in CAS, returned error must be ErrNotFound.
	// In case of returning a stream, caller must ensure to call Close on it
	// after reading it's contents.
	Open(name string) (io.ReadCloser, error)

	// Save should return write stream that can be used to store new data blob.
	// Caller must call Close on the stream to indicate end of data. If the name
	// of blob won't match the contents, ErrNameMismatch error will be returned
	// and no data will be stored. In case a blob with same name already exists,
	// it can be successfully written as long as it's contents does match
	// the name of the blob.
	Save(name string) (io.WriteCloser, error)

	// Exists does check whether blob of given name exists in CAS. Partially
	// written blobs are equal to non-existing ones.
	Exists(name string) bool

	// Delete tries to remove blob with given name. If blob does not exist
	// (which includes partially written blobs) ErrNotFound will be returned.
	// If blob is being opened at the moment of removal, all opened references
	// to the blob will still be able to read the data but all new interface
	// calls would work just as if the blob was already removed.
	Delete(name string) error
}
