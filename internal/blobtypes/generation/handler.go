package generation

import (
	"io"
)

// WriterInfo contains information required for a writer of a given blob
// necessary to generate proper data stream that passes blob validation.
//
// This info may contain data that should be protected from unauthorized use
// since it allows generation of valid blobs that will be propagated by
// the network.
type WriterInfo []byte

type Handler interface {
	// PrepareNewBlob starts generation of a new blob of given type.
	// Additional data may be needed if the blob name / validation is dependent
	// on the content of the data (e.g. static blobs) but it also can be
	// ignored (e.g. dynamic links) adn thus the called must not rely on this method
	// to read the whole dataset.
	PrepareNewBlob(data io.Reader) (hash []byte, wi WriterInfo, err error)

	// SendUpdate takes given input data and writes complete update stream through given output sink
	SendUpdate(hash []byte, wi WriterInfo, data io.Reader, out io.Writer) error
}
