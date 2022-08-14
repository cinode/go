package datastore

import (
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func allTestStorages(t *testing.T) []storage {
	return []storage{
		temporaryFS(t),
		temporaryMemory(t),
	}
}

func TestStorageOpenFailureNotFound(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {
			r, err := st.openReadStream(emptyBlobName)
			require.ErrorIs(t, err, ErrNotFound)
			require.Nil(t, r)
		})
	}
}

func TestStorageSaveOpenSuccess(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {
			exists, err := st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			w, err := st.openWriteStream(emptyBlobName)
			require.NoError(t, err)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			n, err := w.Write([]byte("Hello world!"))
			require.NoError(t, err)
			require.Equal(t, 12, n)

			err = w.Close()
			require.NoError(t, err)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.True(t, exists)

			r, err := st.openReadStream(emptyBlobName)
			require.NoError(t, err)

			b, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, []byte("Hello world!"), b)

			err = r.Close()
			require.NoError(t, err)
		})
	}
}

func TestStorageSaveOpenCancelSuccess(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {
			exists, err := st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			w, err := st.openWriteStream(emptyBlobName)
			require.NoError(t, err)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			n, err := w.Write([]byte("Hello world!"))
			require.NoError(t, err)
			require.Equal(t, 12, n)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			w.Cancel()

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			r, err := st.openReadStream(emptyBlobName)
			require.ErrorIs(t, err, ErrNotFound)
			require.Nil(t, r)
		})
	}
}

func TestStorageDelete(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {
			blobNames := []BlobName{}
			blobDatas := [][]byte{}

			t.Run("generate test data", func(t *testing.T) {
				for _, d := range []string{
					"first",
					"second",
					"third",
				} {
					h := sha256.Sum256([]byte(d))
					bn, err := BlobNameFromHashAndType(h[:], 0x00)
					require.NoError(t, err)

					blobNames = append(blobNames, bn)
					blobDatas = append(blobDatas, []byte(d))

					err = st.delete(bn)
					require.ErrorIs(t, err, ErrNotFound)

					w, err := st.openWriteStream(bn)
					require.NoError(t, err)

					exists, err := st.exists(bn)
					require.NoError(t, err)
					require.False(t, exists)

					n, err := w.Write([]byte(d))
					require.NoError(t, err)
					require.Equal(t, len(d), n)

					err = w.Close()
					require.NoError(t, err)

					exists, err = st.exists(bn)
					require.NoError(t, err)
					require.True(t, exists)
				}
			})

			t.Run("delete blob", func(t *testing.T) {
				const toDelete = 1
				err := st.delete(blobNames[toDelete])
				require.NoError(t, err)

				err = st.delete(blobNames[toDelete])
				require.ErrorIs(t, err, ErrNotFound)

				for i := range blobNames {
					t.Run(fmt.Sprintf("exists test %d", i), func(t *testing.T) {
						exists, err := st.exists(blobNames[i])
						require.NoError(t, err)
						require.Equal(t, i != toDelete, exists)
					})
				}
			})

		})
	}
}

func TestStorageTooManySimultaneousSaves(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {

			// Start the first writer
			w1, err := st.openWriteStream(emptyBlobName)
			require.NoError(t, err)

			// Any attempt to update while the update is in progress should fail now
			w2, err := st.openWriteStream(emptyBlobName)
			require.ErrorIs(t, err, ErrUploadInProgress)
			require.Nil(t, w2)

			// Finish the original ingestion
			err = w1.Close()
			require.NoError(t, err)

			// We should be able to successfully read the ingested data
			r, err := st.openReadStream(emptyBlobName)
			require.NoError(t, err)

			b, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, []byte{}, b)

			err = r.Close()
			require.NoError(t, err)
		})
	}
}

func TestStorageSaveWhileDeleting(t *testing.T) {
	for _, st := range allTestStorages(t) {
		t.Run(st.kind(), func(t *testing.T) {

			w, err := st.openWriteStream(emptyBlobName)
			require.NoError(t, err)

			err = w.Close()
			require.NoError(t, err)

			exists, err := st.exists(emptyBlobName)
			require.NoError(t, err)
			require.True(t, exists)

			w, err = st.openWriteStream(emptyBlobName)
			require.NoError(t, err)

			err = st.delete(emptyBlobName)
			require.NoError(t, err)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.False(t, exists)

			err = w.Close()
			require.NoError(t, err)

			exists, err = st.exists(emptyBlobName)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}
