package datastore

import (
	"errors"
	"io"
)

type datastore struct {
	s storage
}

var _ DS = (*datastore)(nil)

func (ds *datastore) Kind() string {
	return ds.s.kind()
}

func (ds *datastore) Read(name BlobName, output io.Writer) error {
	handler, err := handlerForType(name.Type())
	if err != nil {
		return err
	}

	rc, err := ds.s.openReadStream(name)
	if err != nil {
		return err
	}
	defer rc.Close()

	return handler.Validate(
		name.Hash(),
		rc,
		output,
	)
}

func (ds *datastore) Update(name BlobName, updateStream io.Reader) error {
	handler, err := handlerForType(name.Type())
	if err != nil {
		return err
	}

	outputStream, err := ds.s.openWriteStream(name)
	if err != nil {
		return err
	}
	defer func() {
		// Cancel the write operation if for whatever reason
		// we don't finish the write operation
		if outputStream != nil {
			outputStream.Cancel()
		}
	}()

	// Check if we can get the currentStream blob data
	currentStream, err := ds.s.openReadStream(name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	defer func() {
		if currentStream != nil {
			currentStream.Close()
		}
	}()

	if currentStream == nil {
		// No content yet, only need to validate the updateStream
		// and store in the result file
		err := handler.Validate(name.Hash(), updateStream, outputStream)
		if err != nil {
			return err
		}
	} else {
		err = handler.Ingest(name.Hash(), currentStream, updateStream, outputStream)
		if err != nil {
			return err
		}

		// Current dataset must be closed before closing the output stream
		currentStream.Close()
		currentStream = nil
	}

	err = outputStream.Close()
	if err != nil {
		return err
	}

	outputStream = nil
	return nil
}

func (ds *datastore) Exists(name BlobName) (bool, error) {
	return ds.s.exists(name)
}

func (ds *datastore) Delete(name BlobName) error {
	return ds.s.delete(name)
}

func InMemory() DS {
	return &datastore{s: newStorageMemory()}
}

func InFileSystem(path string) DS {
	return &datastore{s: newStorageFilesystem(path)}
}
