package blenc

import (
	"errors"
	"io"

	"../datastore"
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

func (be *beDatastore) Save(r io.ReadCloser, kg KeyGenerator) (name, key string, err error) {
	key, r2, err := kg.GenerateKey(r)
	if err != nil {
		r.Close()
		return "", "", err
	}

	name, err = be.ds.SaveAutoNamed(r2)
	if err != nil {
		r2.Close()
		return "", "", err
	}

	return name, key, nil
}

func (be *beDatastore) Exists(name string) (bool, error) {
	return false, errors.New("Unimplemented")
}

func (be *beDatastore) Delete(name string) error {
	return errors.New("Unimplemented")
}
