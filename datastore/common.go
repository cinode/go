package datastore

import (
	"crypto/subtle"
	"io"
)

func nameCheckForSave(expectedName string) func(string) bool {
	return func(producedName string) bool {
		return subtle.ConstantTimeCompare(
			[]byte(expectedName),
			[]byte(producedName),
		) == 1
	}
}

type hashValidatingReaderStruct struct {
	rc io.ReadCloser
	rt io.Reader
	hs *hasher
	nm string
}

func (h *hashValidatingReaderStruct) Read(b []byte) (int, error) {
	n, err := h.rt.Read(b)
	if err == io.EOF {
		if subtle.ConstantTimeCompare(
			[]byte(h.nm),
			[]byte(h.hs.Name()),
		) != 1 {
			return 0, ErrNameMismatch
		}
	}
	return n, err
}
func (h *hashValidatingReaderStruct) Close() error {
	return h.rc.Close()
}

func hashValidatingReader(r io.ReadCloser, name string) io.ReadCloser {
	hasher := newHasher()
	rt := io.TeeReader(r, hasher)
	return &hashValidatingReaderStruct{
		rc: r,
		rt: rt,
		hs: hasher,
		nm: name,
	}
}
