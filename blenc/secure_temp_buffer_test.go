package blenc

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestSecureTempBufferReadback(t *testing.T) {
	for _, d := range []struct {
		data []byte
	}{
		{data: []byte{}},
		{data: []byte("a")},
		{data: []byte(strings.Repeat("a", 15))},
		{data: []byte(strings.Repeat("a", 16))},
		{data: []byte(strings.Repeat("a", 17))},
		{data: []byte(strings.Repeat("a", 16*1024))},
	} {
		stb, err := newSecureTempBuffer()
		errPanic(err)
		n, err := stb.Write(d.data)
		errPanic(err)
		if n != len(d.data) {
			t.Fatalf("Invalid number of bytes written: %v instead of %v",
				n, len(d.data))
		}

		rdr := stb.Reader()
		readBack, err := ioutil.ReadAll(rdr)
		errPanic(err)

		if !bytes.Equal(readBack, d.data) {
			t.Fatal("Invalid data read back")
		}

		errPanic(rdr.Close())
		errPanic(stb.Close())
	}
}

func TestSecureTempBufferEarlyClose(t *testing.T) {
	stb, err := newSecureTempBuffer()
	errPanic(err)
	fName := stb.file.Name()
	if _, err := os.Stat(fName); err != nil {
		t.Fatalf("Couldn't ensure file's existance: %v", err)
	}
	stb.Close()
	if _, err := os.Stat(fName); !os.IsNotExist(err) {
		t.Fatalf("Couldn't ensure file's absence: %v", err)
	}
}

func TestSecureTempBufferReaderClose(t *testing.T) {
	stb, err := newSecureTempBuffer()
	errPanic(err)
	fName := stb.file.Name()
	if _, err := os.Stat(fName); err != nil {
		t.Fatalf("Couldn't ensure file's existance: %v", err)
	}
	rdr := stb.Reader()
	stb.Close()
	// If there's a reader, Close won't remove the file
	if _, err := os.Stat(fName); err != nil {
		t.Fatalf("Couldn't ensure file's existance: %v", err)
	}
	rdr.Close()
	if _, err := os.Stat(fName); !os.IsNotExist(err) {
		t.Fatalf("Couldn't ensure file's absence: %v", err)
	}
}

func TestSecureTempBufferData(t *testing.T) {
	data := []byte("testdata")
	stb, err := newSecureTempBuffer()
	errPanic(err)
	fName := stb.file.Name()
	stb.Write(data)

	storedData, err := ioutil.ReadFile(fName)
	errPanic(err)

	if bytes.Equal(storedData, data) {
		t.Fatal("Secure temp buffer does store plaintext data")
	}

	stb.Close()
}

type errorReader struct{}

var errorReaderError = errors.New("Error")

func (e errorReader) Read(_ []byte) (int, error) {
	return 0, errorReaderError
}

func TestSecureTempBufferDataRandError(t *testing.T) {
	randSave := rand.Reader
	defer func() { rand.Reader = randSave }()
	rand.Reader = &errorReader{}

	stb, err := newSecureTempBuffer()
	if err == nil {
		t.Fatalf("Did not get error when no random source is available")
	}

	if stb != nil {
		t.Fatalf("Did receive secure temp buffer object even if error received")
	}
}
