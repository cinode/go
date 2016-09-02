package blenc

import (
	"errors"
	"io"

	"github.com/cinode/go/datastore"
)

var (
	// ErrInvalidKeyDataGenerator is used when given key data generator produces
	// invalid data
	ErrInvalidKeyDataGenerator = errors.New("Invalid key data generator")
)

// FromDatastore creates Blob Encoder using given datastore implementation as
// the storage layer
func FromDatastore(ds datastore.DS) BE {
	return &beDatastore{ds: ds}
}

type beDatastore struct {
	ds datastore.DS
}

func (be *beDatastore) Open(name, key string) (io.ReadCloser, error) {
	return nil, errors.New("Unimplemented")
}

func (be *beDatastore) Save(r io.ReadCloser, kg KeyDataGenerator) (name, key string, err error) {
	keyData, r2, err := kg.GenerateKeyData(r)
	if err != nil {
		r.Close()
		return "", "", err
	}

	if len(keyData) < 32 {
		return "", "", ErrInvalidKeyDataGenerator
	}

	var keyType byte = keyTypeDefault
	r3, err := streamCipherReaderForKeyData(keyType, keyData, r2)
	if err != nil {
		r2.Close()
		return "", "", err
	}

	name, err = be.ds.SaveAutoNamed(r3)
	if err != nil {
		r3.Close()
		return "", "", err
	}

	key = keyFromKeyData(keyType, keyData)

	return name, key, nil
}

func (be *beDatastore) Exists(name string) (bool, error) {
	return be.ds.Exists(name)
}

func (be *beDatastore) Delete(name string) error {
	return be.ds.Delete(name)
}
