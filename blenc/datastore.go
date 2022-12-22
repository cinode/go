package blenc

import (
	"context"
	"crypto/aes"
	"crypto/sha256"
	"io"

	"github.com/cinode/go/common"
	"github.com/cinode/go/datastore"
	"github.com/cinode/go/internal/blobtypes/generation"
	"github.com/cinode/go/internal/utilities/securefifo"
)

// FromDatastore creates Blob Encoder using given datastore implementation as
// the storage layer
func FromDatastore(ds datastore.DS) BE {
	return &beDatastore{ds: ds}
}

type beDatastore struct {
	ds datastore.DS
}

func (be *beDatastore) Read(ctx context.Context, name common.BlobName, ki KeyInfo, w io.Writer) error {
	// TODO: Some analysis of the data from blob types ?
	// In case of dynamic link we'll have to skip link's additional data

	cw, err := streamCipherWriterForKeyInfo(ki, w)
	if err != nil {
		return err
	}

	err = be.ds.Read(ctx, name, cw)
	if err != nil {
		return err
	}
	return nil
}

func (be *beDatastore) Create(ctx context.Context, blobType common.BlobType, r io.Reader) (common.BlobName, KeyInfo, WriterInfo, error) {

	handler, err := generation.HandlerForType(blobType)
	if err != nil {
		return nil, nil, nil, err
	}

	ki, rClone, err := be.generateKeyInfo(r)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rClone.Close()

	tempWriteBuffer, err := securefifo.New()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tempWriteBuffer.Close()

	// Encrypt data with calculated key
	encWriter, err := streamCipherWriterForKeyInfo(ki, tempWriteBuffer)
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = io.Copy(encWriter, rClone)
	if err != nil {
		return nil, nil, nil, err
	}

	encReader, err := tempWriteBuffer.Done()
	if err != nil {
		return nil, nil, nil, err
	}
	defer encReader.Close()

	// Generate new blob info
	hash, wi, err := handler.PrepareNewBlob(encReader)
	if err != nil {
		return nil, nil, nil, err
	}

	name, err := common.BlobNameFromHashAndType(hash, blobType)
	if err != nil {
		return nil, nil, nil, err
	}

	// Store encrypted blob into the datastore
	encReader, err = encReader.Reset()
	if err != nil {
		return nil, nil, nil, err
	}
	defer encReader.Close()

	err = be.ds.Update(ctx, name, encReader)
	if err != nil {
		return nil, nil, nil, err
	}

	return name, ki, wi, nil
}

func (be *beDatastore) generateKeyInfo(r io.Reader) (KeyInfo, io.ReadCloser, error) {

	w, err := securefifo.New()
	if err != nil {
		return nil, nil, err
	}
	defer w.Close()

	// TODO: This should be a part of internal/blobtypes/generation

	// TODO: Those values MUST be checked when reading the blob
	//       to ensure that those can not be weakened on purpose

	// hasher for key
	hasher := sha256.New()
	hasher.Write([]byte{0x00})

	// hasher for IV
	hasher2 := sha256.New()
	hasher2.Write([]byte{0xFF})

	// Copy to temporary fifo and calculate hash at the same time
	_, err = io.Copy(
		w,
		io.TeeReader(io.TeeReader(r, hasher), hasher2),
	)
	if err != nil {
		return nil, nil, err
	}

	hash := hasher.Sum(nil)
	hash2 := hasher2.Sum(nil)

	ki := &keyInfoStatic{
		t:   keyTypeAES,
		key: hash[:keySizeAES],
		iv:  hash2[:aes.BlockSize],
	}

	reader, err := w.Done()
	if err != nil {
		return nil, nil, err
	}

	return ki, reader, nil

}

func (be *beDatastore) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	return be.ds.Exists(ctx, name)
}

func (be *beDatastore) Delete(ctx context.Context, name common.BlobName) error {
	return be.ds.Delete(ctx, name)
}
