package datastore

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockStore struct {
	fKind            func() string
	fOpenReadStream  func(name BlobName) (io.ReadCloser, error)
	fOpenWriteStream func(name BlobName) (WriteCloseCanceller, error)
	fExists          func(name BlobName) (bool, error)
	fDelete          func(name BlobName) error
}

func (s *mockStore) kind() string {
	return s.fKind()
}
func (s *mockStore) openReadStream(name BlobName) (io.ReadCloser, error) {
	return s.fOpenReadStream(name)
}
func (s *mockStore) openWriteStream(name BlobName) (WriteCloseCanceller, error) {
	return s.fOpenWriteStream(name)
}
func (s *mockStore) exists(name BlobName) (bool, error) {
	return s.fExists(name)
}
func (s *mockStore) delete(name BlobName) error {
	return s.fDelete(name)
}

type mockWriteCloseCanceller struct {
	fWrite  func([]byte) (int, error)
	fClose  func() error
	fCancel func()
}

func (m *mockWriteCloseCanceller) Write(b []byte) (int, error) {
	return m.fWrite(b)
}
func (m *mockWriteCloseCanceller) Close() error {
	return m.fClose()
}
func (m *mockWriteCloseCanceller) Cancel() {
	m.fCancel()
}

func TestDatastoreWriteFailure(t *testing.T) {
	t.Run("error on opening write stream", func(t *testing.T) {
		errRet := errors.New("error")
		ds := &datastore{s: &mockStore{
			fOpenWriteStream: func(name BlobName) (WriteCloseCanceller, error) {
				return nil, errRet
			},
		}}

		err := ds.Update(emptyBlobName, bytes.NewBuffer(nil))
		require.ErrorIs(t, err, errRet)
	})

	t.Run("error on opening current data", func(t *testing.T) {
		errRet := errors.New("error")
		cancelCalled := false
		ds := &datastore{s: &mockStore{
			fOpenWriteStream: func(name BlobName) (WriteCloseCanceller, error) {
				return &mockWriteCloseCanceller{
					fCancel: func() {
						require.False(t, cancelCalled)
						cancelCalled = true
					},
				}, nil
			},
			fOpenReadStream: func(name BlobName) (io.ReadCloser, error) {
				return nil, errRet
			},
		}}

		err := ds.Update(emptyBlobName, bytes.NewBuffer(nil))
		require.ErrorIs(t, err, errRet)

		require.True(t, cancelCalled)
	})

	t.Run("error on closing write stream", func(t *testing.T) {
		errRet := errors.New("error")

		closeCalled := false
		cancelCalled := false
		ds := &datastore{s: &mockStore{
			fOpenWriteStream: func(name BlobName) (WriteCloseCanceller, error) {
				return &mockWriteCloseCanceller{
					fWrite: func(b []byte) (int, error) {
						require.False(t, closeCalled)
						return len(b), nil
					},
					fClose: func() error {
						require.False(t, closeCalled)
						require.False(t, cancelCalled)
						closeCalled = true
						return errRet
					},
					fCancel: func() {
						require.True(t, closeCalled)
						require.False(t, cancelCalled)
						cancelCalled = true
					},
				}, nil
			},
			fOpenReadStream: func(name BlobName) (io.ReadCloser, error) {
				return nil, ErrNotFound
			},
		}}

		err := ds.Update(emptyBlobName, bytes.NewBuffer(nil))
		require.ErrorIs(t, err, errRet)

		// Failed Close call will be followed by a Cancel call
		require.True(t, closeCalled)
		require.True(t, cancelCalled)
	})
}
