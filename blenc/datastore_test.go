package blenc

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cinode/go/datastore"
)

type testBogusKeyGenerator struct{}

var errBogusKeyGeneratorError = errors.New("bogusKeyGeneratorError")

func (t testBogusKeyGenerator) IsDeterministic() bool {
	return true
}

func (t testBogusKeyGenerator) GenerateKeyData(io.ReadCloser) (keyData []byte, origStream io.ReadCloser, err error) {
	err = errBogusKeyGeneratorError
	return
}

func TestSaveWithBogusKeyGenerator(t *testing.T) {
	bogusKG := &testBogusKeyGenerator{}
	allBE(func(be BE) {
		closeCalled := false
		name, key, err := be.Save(bReader([]byte{}, nil, nil, func() error {
			if closeCalled {
				t.Fatalf("Multiple close called")
			}
			closeCalled = true
			return nil
		}), bogusKG)
		if err != errBogusKeyGeneratorError {
			t.Fatalf("Invalid error received for bogus key generator: %v", err)
		}
		if name != "" {
			t.Fatalf("Non-empty name received for bogus key generator: %v", name)
		}
		if key != "" {
			t.Fatalf("Non-empty key received for bogus key generator: %v", key)
		}
		if !closeCalled {
			t.Fatal("Input stream was not closed for bogus key generator")
		}
	})
}

func TestSaveWithBogusKeyGenerator2(t *testing.T) {
	// Low amount of data in the key generator - must fail
	bogusKG := constantKey([]byte{0x01, 0x02})
	allBE(func(be BE) {
		closeCalled := false
		name, key, err := be.Save(bReader([]byte{}, nil, nil, func() error {
			if closeCalled {
				t.Fatalf("Multiple close called")
			}
			closeCalled = true
			return nil
		}), bogusKG)
		if err != ErrInvalidKey {
			t.Fatalf("Invalid error received for bogus key generator: %v", err)
		}
		if name != "" {
			t.Fatalf("Non-empty name received for bogus key generator: %v", name)
		}
		if key != "" {
			t.Fatalf("Non-empty key received for bogus key generator: %v", key)
		}
		if !closeCalled {
			t.Fatal("Input stream was not closed for bogus key generator")
		}
	})
}

func TestSaveErrorWhileReading(t *testing.T) {
	kg := constantKey([]byte(strings.Repeat("*", 32)))
	allBE(func(be BE) {
		closeCalled := false
		errToReturn := errors.New("You shall not pass`")
		name, key, err := be.Save(bReader([]byte{}, func() error {
			return errToReturn
		}, nil, func() error {
			if closeCalled {
				t.Fatalf("Multiple close called")
			}
			closeCalled = true
			return nil
		}), kg)
		if err != errToReturn {
			t.Fatalf("Invalid error received on read error: %v", err)
		}
		if name != "" {
			t.Fatalf("Non-empty name received on read error: %v", name)
		}
		if key != "" {
			t.Fatalf("Non-empty key received on read error: %v", key)
		}
		if !closeCalled {
			t.Fatal("Input stream was not closed on read error")
		}
	})
}

func beSave(be BE, data string) (name string, key string) {
	kg := constantKey([]byte(strings.Repeat("*", 32)))
	name, key, err := be.Save(bReader([]byte(data), nil, nil, nil), kg)
	errPanic(err)
	return name, key
}

func beExists(be BE, name string) bool {
	ret, err := be.Exists(name)
	errPanic(err)
	return ret
}

func TestExistsDelete(t *testing.T) {

	allBE(func(be BE) {

		name, _ := beSave(be, "Test1")
		if !beExists(be, name) {
			t.Fatalf("Blob should exist")
		}

		err := be.Delete(name)
		if err != nil {
			t.Fatalf("Could not delete blog: %v", err)
		}
		err = be.Delete(name)
		if err != datastore.ErrNotFound {
			t.Fatalf("Double delete returned invalid error: %v", err)
		}

	})
}
