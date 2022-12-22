/*
Copyright © 2022 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package securefifo

import (
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"

	"golang.org/x/crypto/chacha20"
)

type closingStreamReader struct {
	cipher.StreamReader
}

func (r *closingStreamReader) Close() error {
	if c, ok := r.R.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

type SecureFifoReader interface {
	io.ReadCloser

	// Reset closes current reader and opens a new one that starts at the beginning of the data
	Reset() (SecureFifoReader, error)
}

type SecureFifoWriter interface {
	io.WriteCloser

	// Done closes current writer and opens SecureFifoReader stream for reading
	Done() (SecureFifoReader, error)
}

type secureFifo struct {
	fl *os.File

	key   []byte
	nonce []byte
}

func (f *secureFifo) Close() error {
	return f.fl.Close()
}

func (f *secureFifo) getStream() cipher.Stream {
	stream, _ := chacha20.NewUnauthenticatedCipher(f.key, f.nonce)
	return stream
}

func (f *secureFifo) openReader() (*secureFifoReader, error) {
	_, err := f.fl.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	return &secureFifoReader{
		sf: f,
		r:  cipher.StreamReader{S: f.getStream(), R: f.fl},
	}, nil
}

type secureFifoWriter struct {
	sf *secureFifo
	w  io.Writer
}

func (w *secureFifoWriter) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *secureFifoWriter) Close() error {
	if w.sf == nil {
		return nil
	}
	defer func() { w.sf = nil; w.w = nil }()
	return w.sf.Close()
}

func (w *secureFifoWriter) Done() (SecureFifoReader, error) {
	ret, err := w.sf.openReader()
	if err != nil {
		return nil, err
	}

	w.sf = nil
	w.w = nil

	return ret, nil
}

type secureFifoReader struct {
	sf *secureFifo
	r  io.Reader
}

func (r *secureFifoReader) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

func (r *secureFifoReader) Close() error {
	if r.sf == nil {
		return nil
	}
	defer func() { r.sf = nil; r.r = nil }()
	return r.sf.Close()
}

func (r *secureFifoReader) Reset() (SecureFifoReader, error) {
	ret, err := r.sf.openReader()
	if err != nil {
		return nil, err
	}

	r.sf = nil
	r.r = nil

	return ret, nil
}

// New creates new secure fifo pipe. That pipe may handle large amounts of data by using a temporary storage
// but ensures that even if the data can be accessed from disk, it can not be decrypted.
func New() (wr SecureFifoWriter, err error) {

	var randData [chacha20.KeySize + chacha20.NonceSize]byte
	_, err = rand.Read(randData[:])
	if err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp("", "secure-fifo")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}
	}()

	// Supported on Linux (to check on Mac OSX) - unlinking already opened file
	// will still allow reading / writing that file by using already opened handles
	err = os.Remove(tempFile.Name())
	if err != nil {
		return nil, err
	}

	sf := &secureFifo{
		key:   randData[:chacha20.KeySize],
		nonce: randData[chacha20.KeySize:],
		fl:    tempFile,
	}

	return &secureFifoWriter{
		sf: sf,
		w:  cipher.StreamWriter{S: sf.getStream(), W: tempFile},
	}, nil
}
