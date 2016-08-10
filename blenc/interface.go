package blenc

import "io"

// KeyGenerationMode is used to specify how encryption key should be generated
type KeyGenerationMode int

const (
	// KGMRandom is used to specify that encryption key should be generated
	// randomly
	KGMRandom KeyGenerationMode = iota

	// KGMDerivedDeterministic is used to specify that encryption key should be
	// derived from hash of blob's contents
	KGMDerivedDeterministic
)

// BE interface describes functoinality exposed by Blob Encryption layer
// implementation
type BE interface {

	// Open returns a read stream for given blob name or an error. In case blob
	// is not found in datastore, returned error must be ErrNotFound.
	// In case of returning a stream, caller must ensure to call Close on it
	// after reading it's contents.
	Open(name, key string) (io.ReadCloser, error)

	// Save gathers data from given ReadCloser interface and stores it's
	// encrypted version in the underlying Datastore.
	// Key is generated according to given key generation mode.
	Save(r io.ReadCloser, kgm KeyGenerationMode) (name, key string, err error)

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(name string) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(name string) error
}
