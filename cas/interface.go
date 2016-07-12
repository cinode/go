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

	// Save tries to save data blob with given name. Blob's data will be read
	// from given reader until either EOF ending successfull save or any other
	// error which will cancel the save - in such case this error will be
	// returned from this function. If name does not match blob's data,
	// ErrNameMismatch will be returned. In case of either error or success,
	// reader will be closed.
	Save(name string, r io.ReadCloser) error

	// SaveAutoNamed creates new blob but automatically calculates it's name.
	// Apart from the name and lack of ErrNameMismatch, the behavior of this
	// function is equal to Save()
	SaveAutoNamed(r io.ReadCloser) (name string, err error)

	// Exists does check whether blob of given name exists in CAS. Partially
	// written blobs are equal to non-existing ones. If blob exists, returned
	// error will be nil, if blob does not exists, returned value will be
	// ErrNotFound. In case of any other error encountered during blob existance
	// evaluation, appropriate error should be returned.
	Exists(name string) error

	// Delete tries to remove blob with given name. If blob does not exist
	// (which includes partially written blobs) ErrNotFound will be returned.
	// If blob is being opened at the moment of removal, all opened references
	// to the blob will still be able to read the data but all new interface
	// calls would work just as if the blob was already removed.
	Delete(name string) error
}
