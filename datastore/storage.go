package datastore

import "io"

type WriteCloseCanceller interface {
	io.WriteCloser
	Cancel()
}

type storage interface {
	kind() string
	openReadStream(name BlobName) (io.ReadCloser, error)
	openWriteStream(name BlobName) (WriteCloseCanceller, error)
	exists(name BlobName) (bool, error)
	delete(name BlobName) error
}
