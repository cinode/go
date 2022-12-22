package propagation

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func getUpdatedBlob(bt Handler, hash []byte, current, update []byte) ([]byte, error) {
	var currentReader io.Reader
	if current != nil {
		currentReader = bytes.NewReader(current)
	}

	return getUpdatedBlobWithCurrentReader(bt, hash, currentReader, update)
}

func getUpdatedBlobWithCurrentReader(bt Handler, hash []byte, currentReader io.Reader, update []byte) ([]byte, error) {

	output := bytes.NewBuffer(nil)

	err := bt.Ingest(hash[:], currentReader, bytes.NewReader(update), output)
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func TestStaticBlobHandler(t *testing.T) {
	bt := newStaticBlobHandlerSha256()

	t.Run("ingest a new correct blob", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlob(bt, hash[:], nil, data)
		require.NoError(t, err)
		require.Equal(t, data, dataBack)
	})

	t.Run("propagate read error", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)
		errToReturn := errors.New("test error")

		err := bt.Validate(hash[:], iotest.ErrReader(errToReturn))
		require.ErrorIs(t, err, errToReturn)
	})

	t.Run("ingest a new incorrect blob - hash of wrong data", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(append(data, 1))

		dataBack, err := getUpdatedBlob(bt, hash[:], nil, data)
		require.ErrorIs(t, err, ErrInvalidStaticBlobHash)
		require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
		require.Nil(t, dataBack)
	})

	t.Run("ingest a new incorrect blob - hash size mismatch", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlob(bt, hash[:len(hash)-1], nil, data)
		require.ErrorIs(t, err, ErrInvalidStaticBlobHash)
		require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
		require.Nil(t, dataBack)
	})

	t.Run("ingest a correct update", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlob(bt, hash[:], data, data)
		require.NoError(t, err)
		require.Equal(t, data, dataBack)
	})

	t.Run("ingest an incorrect update - hash of wrong data", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		_, err := getUpdatedBlob(bt, hash[:], data, append(data, 1))
		require.ErrorIs(t, err, ErrInvalidStaticBlobHash)
		require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
	})

	t.Run("ingest a new incorrect blob - hash size mismatch", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlob(bt, hash[:len(hash)-1], data, data)
		require.ErrorIs(t, err, ErrInvalidStaticBlobHash)
		require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
		require.Nil(t, dataBack)
	})

	t.Run("recover from a broken local data - hash mismatch", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlob(bt, hash[:], append(data, 1), data)
		require.NoError(t, err)
		require.Equal(t, data, dataBack)
	})

	t.Run("recover from a broken local data - read error", func(t *testing.T) {
		data := []byte("hello world!")
		hash := sha256.Sum256(data)

		dataBack, err := getUpdatedBlobWithCurrentReader(
			bt, hash[:], iotest.ErrReader(errors.New("err")), data,
		)
		require.NoError(t, err)
		require.Equal(t, data, dataBack)
	})

}
