package blenc

import "io"

// KeyGenerator is used to generate key before encrypting blob's data.
type KeyGenerator interface {

	// GenerateKey takes given data stream, calculates symmetric key for it's
	// contents and returns stream with the same data along with the key or an
	// error if the key could not be calculated.
	//
	// Note: Because key genereation can consume stream's data, GenerateKey is
	//       responsible to create needed copies of such data using local storage.
	//       Due to security reasons, this temporary storage must not be stored in
	//       a plaintext form. An encrypted form must be stored instead where keys
	//       would only be held in memory.
	GenerateKey(stream io.ReadCloser) (key string, origStream io.ReadCloser, err error)

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
	Save(r io.ReadCloser, kg KeyGenerator) (name, key string, err error)

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(name string) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(name string) error
}
