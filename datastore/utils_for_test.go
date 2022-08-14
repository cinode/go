package datastore

import (
	"bytes"
	"crypto/sha256"
	"io"
)

var emptyBlobName = func() BlobName {
	bn, err := BlobNameFromHashAndType(sha256.New().Sum(nil), 0x00)
	if err != nil {
		panic(err)
	}
	return bn
}()

func testBlobNameFromString(n string) BlobName {
	bn, err := BlobNameFromString(n)
	if err != nil {
		panic(err)
	}
	return bn
}

var testBlobs = []struct {
	name BlobName
	data []byte
}{
	{testBlobNameFromString("JvNiMF6m1MiYC1zuxnyN8zTwq5nVcTJiQEisbX7vLDfvU"), []byte("Test")},
	{testBlobNameFromString("BZMpx28vDYHQMmzb8X18KzZxxKUou93EwLjcQFxy9WiYE"), []byte("Test1")},
	{testBlobNameFromString("2Ge33RgXs3in9ZFHEYJs8od7pjmgr4cMbbovQ9D3WHLzjv"), []byte("")},
}

type helperReader struct {
	buf    io.Reader
	onRead func() error
	onEOF  func() error
}

func bReader(b []byte, onRead func() error, onEOF func() error) *helperReader {

	nop := func() error {
		return nil
	}

	if onRead == nil {
		onRead = nop
	}
	if onEOF == nil {
		onEOF = nop
	}

	return &helperReader{
		buf:    bytes.NewReader(b),
		onRead: onRead,
		onEOF:  onEOF,
	}
}

func (h *helperReader) Read(b []byte) (n int, err error) {
	err = h.onRead()
	if err != nil {
		return 0, err
	}

	n, err = h.buf.Read(b)
	if err == io.EOF {
		err = h.onEOF()
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	return n, err
}
