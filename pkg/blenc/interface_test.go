package blenc

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/sha256"
	"testing"

	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func TestBlencCommonScenario(t *testing.T) {
	be := FromDatastore(datastore.InMemory())

	data := []byte("Hello world!!!")

	bn, ki, wi, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, blobtypes.Static, bn.Type())
	require.Len(t, bn.Hash(), sha256.Size)
	require.Nil(t, wi) // Static blobs don't generate writer info

	kType, key, iv, err := ki.GetSymmetricKey()
	require.NoError(t, err)
	require.Equal(t, keyTypeAES, kType)
	require.Len(t, key, keySizeAES)
	require.Len(t, iv, aes.BlockSize)

	exists, err := be.Exists(context.Background(), bn)
	require.NoError(t, err)
	require.True(t, exists)

	w := bytes.NewBuffer(nil)
	err = be.Read(context.Background(), bn, ki, w)
	require.NoError(t, err)
	require.Equal(t, data, w.Bytes())

	err = be.Delete(context.Background(), bn)
	require.NoError(t, err)

	exists, err = be.Exists(context.Background(), bn)
	require.NoError(t, err)
	require.False(t, exists)

	data2 := []byte("Hello Cinode!")

	bn2, ki2, wi2, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data2))
	require.NoError(t, err)
	require.NotEqual(t, bn, bn2)
	require.Nil(t, wi2)

	kType2, key2, iv2, err := ki2.GetSymmetricKey()
	require.NoError(t, err)
	require.Equal(t, keyTypeAES, kType2)
	require.NotEqual(t, key, key2)
	require.Equal(t, len(key), len(key2))
	require.NotEqual(t, iv, iv2)
	require.Equal(t, len(iv), len(iv2))
}

// func TestNewBE(t *testing.T) {
// 	testData1 := []byte("data1")
// 	testData2 := []byte("data2" + strings.Repeat("longdata", 1024))

// 	allBEKG(func(be BE, kg KeyDataGenerator) {
// 		d1n, d1k, err := be.Save(bReader(testData1, nil, nil, nil), kg)
// 		errPanic(err)

// 		d1n2, d1k2, err := be.Save(bReader(testData1, nil, nil, nil), kg)
// 		errPanic(err)

// 		if kg.IsDeterministic() {
// 			if d1n != d1n2 {
// 				t.Fatal("Saving identical blobs with deterministic KG produced different blob names")
// 			}
// 			if d1k != d1k2 {
// 				t.Fatal("Saving identical blobs with deterministic KG produced different keys")
// 			}
// 		} else {
// 			if d1n == d1n2 {
// 				t.Fatal("Saving identical blobs with non-deterministic KG produced identical blob names")
// 			}
// 			if d1k == d1k2 {
// 				t.Fatal("Saving identical blobs with non-deterministic KG produced identical keys")
// 			}
// 		}

// 		d2n, d2k, err := be.Save(bReader(testData2, nil, nil, nil), kg)
// 		errPanic(err)
// 		if d2n == d1n || d2n == d1n2 {
// 			t.Fatal("Same data blob name for different contents")
// 		}

// 		for _, d := range []struct {
// 			name string
// 			key  string
// 			data []byte
// 		}{
// 			{d1n, d1k, testData1},
// 			{d1n2, d1k2, testData1},
// 			{d2n, d2k, testData2},
// 		} {
// 			// Test if we can read back the data
// 			stream, err := be.Open(d.name, d.key)
// 			if err != nil {
// 				t.Fatalf("Couldn't open data for reading: %v", err)
// 			}
// 			data, err := io.ReadAll(stream)
// 			if err != nil {
// 				t.Fatalf("Couldn't read data: %v", err)
// 			}
// 			err = stream.Close()
// 			if err != nil {
// 				t.Fatalf("Couldn't close data stream: %v", err)
// 			}
// 			if !bytes.Equal(data, d.data) {
// 				t.Fatal("Read incorrect data back")
// 			}
// 		}
// 	})
// }

// type testBogusKeyGenerator struct{}

// var errBogusKeyGeneratorError = errors.New("bogusKeyGeneratorError")

// func (t testBogusKeyGenerator) IsDeterministic() bool {
// 	return true
// }

// func (t testBogusKeyGenerator) GenerateKeyData(stream io.ReadCloser, keyData []byte) (
// 	sameStream io.ReadCloser, err error) {
// 	err = errBogusKeyGeneratorError
// 	return
// }

// func TestSaveWithBogusKeyGenerator(t *testing.T) {
// 	bogusKG := &testBogusKeyGenerator{}
// 	allBE(func(be BE) {
// 		closeCalled := false
// 		name, key, err := be.Save(bReader([]byte{}, nil, nil, func() error {
// 			if closeCalled {
// 				t.Fatalf("Multiple close called")
// 			}
// 			closeCalled = true
// 			return nil
// 		}), bogusKG)
// 		if err != errBogusKeyGeneratorError {
// 			t.Fatalf("Invalid error received for bogus key generator: %v", err)
// 		}
// 		if name != "" {
// 			t.Fatalf("Non-empty name received for bogus key generator: %v", name)
// 		}
// 		if key != "" {
// 			t.Fatalf("Non-empty key received for bogus key generator: %v", key)
// 		}
// 		if !closeCalled {
// 			t.Fatal("Input stream was not closed for bogus key generator")
// 		}
// 	})
// }

// func TestSaveWithBogusKeyGenerator2(t *testing.T) {
// 	// Low amount of data in the key generator - must fail
// 	bogusKG := constantKey([]byte{0x01, 0x02})
// 	allBE(func(be BE) {
// 		closeCalled := false
// 		name, key, err := be.Save(bReader([]byte{}, nil, nil, func() error {
// 			if closeCalled {
// 				t.Fatalf("Multiple close called")
// 			}
// 			closeCalled = true
// 			return nil
// 		}), bogusKG)
// 		if err != errInsufficientKeyData {
// 			t.Fatalf("Invalid error received for bogus key generator: %v", err)
// 		}
// 		if name != "" {
// 			t.Fatalf("Non-empty name received for bogus key generator: %v", name)
// 		}
// 		if key != "" {
// 			t.Fatalf("Non-empty key received for bogus key generator: %v", key)
// 		}
// 		if !closeCalled {
// 			t.Fatal("Input stream was not closed for bogus key generator")
// 		}
// 	})
// }

// func TestSaveErrorWhileReading(t *testing.T) {
// 	kg := constantKey([]byte(strings.Repeat("*", 32)))
// 	allBE(func(be BE) {
// 		closeCalled := false
// 		errToReturn := errors.New("You shall not pass`")
// 		name, key, err := be.Save(bReader([]byte{}, func() error {
// 			return errToReturn
// 		}, nil, func() error {
// 			if closeCalled {
// 				t.Fatalf("Multiple close called")
// 			}
// 			closeCalled = true
// 			return nil
// 		}), kg)
// 		if err != errToReturn {
// 			t.Fatalf("Invalid error received on read error: %v", err)
// 		}
// 		if name != "" {
// 			t.Fatalf("Non-empty name received on read error: %v", name)
// 		}
// 		if key != "" {
// 			t.Fatalf("Non-empty key received on read error: %v", key)
// 		}
// 		if !closeCalled {
// 			t.Fatal("Input stream was not closed on read error")
// 		}
// 	})
// }

// func TestExistsDelete(t *testing.T) {

// 	allBE(func(be BE) {

// 		name, _ := beSave(be, "Test1")
// 		if !beExists(be, name) {
// 			t.Fatalf("Blob should exist")
// 		}

// 		err := be.Delete(name)
// 		if err != nil {
// 			t.Fatalf("Could not delete blog: %v", err)
// 		}
// 		err = be.Delete(name)
// 		if err != datastore.ErrNotFound {
// 			t.Fatalf("Double delete returned invalid error: %v", err)
// 		}

// 	})
// }

// func TestOpenNonExisting(t *testing.T) {
// 	allBE(func(be BE) {
// 		_, err := be.Open("nonexistingblob", "nomatterwhat")
// 		if err == nil {
// 			t.Fatal("Did not get error while trying to open non-existing blob")
// 		}
// 	})
// }

// func TestOpenWrongKey(t *testing.T) {
// 	allBE(func(be BE) {
// 		name, _ := beSave(be, "Testdata")
// 		_, err := be.Open(name, "invalidkey")
// 		if err == nil {
// 			t.Fatal("Did not get error while trying to use incorrect key")
// 		}
// 	})
// }
