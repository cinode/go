package blenc

import "io"

// KeyDataGenerator is used to generate key before encrypting blob's data.
type KeyDataGenerator interface {

	// GenerateKeyData takes given data stream, calculates set of bytes to be
	// used as a key. This data should be reveal pseudorandom properties - it
	// will be used directly as a key to selected cipher. If needed, one must
	// apply key derivation function on data that's not good enough. Number of
	// bytes returned must be at least 32.
	//
	// Note: Because key genereation can consume stream's data, GenerateKey is
	//       responsible to create needed copies of such data using local storage.
	//       Due to security reasons, this temporary storage must not be stored in
	//       a plaintext form. An encrypted form must be stored instead where keys
	//       would only be held in memory.
	GenerateKeyData(stream io.ReadCloser) (keyData []byte, origStream io.ReadCloser, err error)

	// IsDeterministic returns true if this key generator is deterministic which
	// means that it returns same key for same data.
	IsDeterministic() bool
}

// BE interface describes functionality exposed by Blob Encryption layer
// implementation
type BE interface {

	// Open returns a read stream for given blob name or an error. In case blob
	// is not found in datastore, returned error must be ErrNotFound.
	// In case of returning a stream, caller must ensure to call Close on it
	// after reading it's contents.
	Open(name, key string) (io.ReadCloser, error)

	// Save gathers data from given ReadCloser interface and stores it's
	// encrypted version in the underlying Datastore.
	// Key is generated using given key generator.
	Save(r io.ReadCloser, kg KeyDataGenerator) (name, key string, err error)

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(name string) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(name string) error
}
