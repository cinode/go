package blenc

import (
	"crypto/cipher"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"

	"github.com/yawning/chacha20"
)

type secureTempBuffer struct {
	file *os.File
	sw   io.Writer
	sr   io.Reader
}

func newSecureTempBuffer() (*secureTempBuffer, error) {

	var randData [chacha20.KeySize + chacha20.NonceSize]byte
	_, err := rand.Read(randData[:])
	if err != nil {
		return nil, err
	}

	key, nonce := randData[:chacha20.KeySize], randData[chacha20.KeySize:]

	tempFile, err := ioutil.TempFile("", "cinodeupload")
	if err != nil {
		return nil, err
	}

	cc1, _ := chacha20.NewCipher(key, nonce)
	cc2, _ := chacha20.NewCipher(key, nonce)

	return &secureTempBuffer{
		file: tempFile,
		sw:   &cipher.StreamWriter{S: cc1, W: tempFile},
		sr:   &cipher.StreamReader{S: cc2, R: tempFile},
	}, nil
}

func (s *secureTempBuffer) Write(b []byte) (int, error) {
	return s.sw.Write(b)
}

func (s *secureTempBuffer) Close() error {
	if s.file != nil {
		s.file.Close()
		os.Remove(s.file.Name())
		s.file = nil
	}
	return nil
}

type secureTempBufferReader struct {
	file *os.File
	sr   io.Reader
}

func (r *secureTempBufferReader) Read(b []byte) (n int, err error) {
	return r.sr.Read(b)
}

func (r *secureTempBufferReader) Close() error {
	r.file.Close()
	return os.Remove(r.file.Name())
}

func (s *secureTempBuffer) Reader() io.ReadCloser {
	reader := &secureTempBufferReader{
		file: s.file,
		sr:   s.sr,
	}
	s.file.Seek(0, os.SEEK_SET)
	s.file, s.sw, s.sr = nil, nil, nil
	return reader
}
