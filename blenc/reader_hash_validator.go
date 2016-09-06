package blenc

import (
	"crypto/subtle"
	"errors"
	"hash"
	"io"
)

var errHashValidationError = errors.New("Hash of the data does not match the expected value")

type readerHashValidator struct {
	rc   io.ReadCloser
	rt   io.Reader
	h    hash.Hash
	hexp []byte
}

func (r *readerHashValidator) Read(b []byte) (n int, err error) {
	n, err = r.rt.Read(b)
	if err == io.EOF {
		// All data read, need to test the hash
		if subtle.ConstantTimeCompare(r.h.Sum(nil), r.hexp) == 0 {
			return 0, errHashValidationError
		}
	}
	return n, err
}

func (r *readerHashValidator) Close() error {
	return r.rc.Close()
}

func newReaderHashValidator(rc io.ReadCloser, h hash.Hash, hexp []byte) io.ReadCloser {
	return &readerHashValidator{
		rc:   rc,
		rt:   io.TeeReader(rc, h),
		h:    h,
		hexp: hexp,
	}
}
