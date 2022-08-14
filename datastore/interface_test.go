package datastore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func allTestInterfaces(t *testing.T) []DS {

	// Test web interface and web connector
	server := httptest.NewServer(WebInterface(InMemory()))
	t.Cleanup(func() { server.Close() })

	return []DS{
		InMemory(),
		InFileSystem(t.TempDir()),
		FromWeb(server.URL+"/", &http.Client{}),
	}
}

func TestOpenNonExisting(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			err := ds.Read(emptyBlobName, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestOpenInvalidBlobType(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			bn, err := BlobNameFromHashAndType(sha256.New().Sum(nil), 0xFF)
			require.NoError(t, err)

			err = ds.Read(bn, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrUnknownBlobType)

			err = ds.Update(bn, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrUnknownBlobType)
		})
	}
}

func TestSaveNameMismatch(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			err := ds.Update(emptyBlobName, bytes.NewReader([]byte("test")))
			require.ErrorIs(t, err, ErrValidationFailed)
		})
	}
}

func TestSaveSuccessful(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {

			for _, b := range testBlobs {

				exists, err := ds.Exists(b.name)
				require.NoError(t, err)
				require.False(t, exists)

				err = ds.Update(b.name, bytes.NewReader(b.data))
				require.NoError(t, err)

				exists, err = ds.Exists(b.name)
				require.NoError(t, err)
				require.True(t, exists)

				// Overwrite with the same data must be fine
				err = ds.Update(b.name, bytes.NewReader(b.data))
				require.NoError(t, err)

				exists, err = ds.Exists(b.name)
				require.NoError(t, err)
				require.True(t, exists)

				// Overwrite with wrong data must fail
				err = ds.Update(b.name, bytes.NewReader(append([]byte{0x00}, b.data...)))
				require.ErrorIs(t, err, ErrValidationFailed)

				exists, err = ds.Exists(b.name)
				require.NoError(t, err)
				require.True(t, exists)

				data := bytes.NewBuffer(nil)
				err = ds.Read(b.name, data)
				require.NoError(t, err)
				require.Equal(t, b.data, data.Bytes())
			}
		})
	}
}

func TestErrorWhileUpdating(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			for _, b := range testBlobs {
				errRet := errors.New("Test error")
				err := ds.Update(b.name, bReader(b.data, func() error {
					return errRet
				}, nil))
				require.ErrorIs(t, err, errRet)

				exists, err := ds.Exists(b.name)
				require.NoError(t, err)
				require.False(t, exists)
			}
		})
	}
}

func TestErrorWhileOverwriting(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			b := testBlobs[0]

			err := ds.Update(b.name, bytes.NewReader(b.data))
			require.NoError(t, err)

			errRet := errors.New("cancel")

			err = ds.Update(b.name, bReader(b.data, func() error {
				exists, err := ds.Exists(b.name)
				require.NoError(t, err)
				require.True(t, exists)

				return errRet
			}, nil))

			require.ErrorIs(t, err, errRet)

			exists, err := ds.Exists(b.name)
			require.NoError(t, err)
			require.True(t, exists)

			data := bytes.NewBuffer(nil)
			err = ds.Read(b.name, data)
			require.NoError(t, err)
			require.Equal(t, b.data, data.Bytes())
		})
	}
}

func TestDeleteNonExisting(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			b := testBlobs[0]

			err := ds.Update(b.name, bytes.NewReader(b.data))
			require.NoError(t, err)

			err = ds.Delete(testBlobs[1].name)
			require.ErrorIs(t, err, ErrNotFound)

			exists, err := ds.Exists(b.name)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}

// func TestDeleteExisting(t *testing.T) {
// 	allDS(func(c DS) {

// 		b := testBlobs[0]
// 		putBlob(b.name, b.data, c)

// 		if !exists(c, b.name) {
// 			t.Fatalf("Datastore %s: Blob should exist", c.Kind())
// 		}

// 		if !bytes.Equal(b.data, getBlob(b.name, c)) {
// 			t.Fatalf("Datastore %s: Did read invalid data", c.Kind())
// 		}

// 		err := c.Delete(b.name)
// 		if err != nil {
// 			t.Fatalf("Datastore %s: Couldn't delete blob: %v", c.Kind(), err)
// 		}

// 		if exists(c, b.name) {
// 			t.Fatalf("Datastore %s: Blob should not exist", c.Kind())
// 		}

// 		r, err := c.Open(b.name)
// 		if err != ErrNotFound {
// 			t.Fatalf("Datastore %s: Did not get ErrNotFound error after blob deletion", c.Kind())
// 		}
// 		if r != nil {
// 			t.Fatalf("Datastore %s: Got reader for deleted blob", c.Kind())
// 		}

// 	})
// }

// func TestGetKind(t *testing.T) {
// 	allDS(func(c DS) {
// 		k := c.Kind()
// 		if len(k) == 0 {
// 			t.Fatalf("Invalid kind - empty string")
// 		}
// 	})
// }

// func TestSimultaneousReads(t *testing.T) {
// 	threadCnt := 10
// 	readCnt := 200

// 	allDS(func(c DS) {

// 		// Prepare data
// 		for _, b := range testBlobs {
// 			putBlob(b.name, b.data, c)
// 		}

// 		wg := sync.WaitGroup{}
// 		wg.Add(threadCnt)

// 		for i := 0; i < threadCnt; i++ {
// 			go func(i int) {
// 				defer wg.Done()
// 				for n := 0; n < readCnt; n++ {
// 					b := testBlobs[(i+n)%len(testBlobs)]
// 					if !bytes.Equal(b.data, getBlob(b.name, c)) {
// 						t.Errorf("Datastore %s: Did read invalid data", c.Kind())
// 						break
// 					}
// 				}
// 			}(i)
// 		}

// 		wg.Wait()
// 	})
// }

// func TestSimultaneousSaves(t *testing.T) {
// 	threadCnt := 3

// 	allDS(func(c DS) {

// 		b := testBlobs[0]

// 		wg := sync.WaitGroup{}
// 		wg.Add(threadCnt)

// 		wg2 := sync.WaitGroup{}
// 		wg2.Add(threadCnt)

// 		for i := 0; i < threadCnt; i++ {
// 			go func(i int) {
// 				firstTime := true
// 				err := c.Save(b.name, bReader(b.data, func() error {

// 					if !firstTime {
// 						return nil
// 					}
// 					firstTime = false

// 					// Blob must not exist now
// 					if exists(c, b.name) {
// 						t.Errorf("Datastore %s: Blob exists although no writter finished yet", c.Kind())
// 						return nil
// 					}

// 					// Wait for all writes to start
// 					wg.Done()
// 					wg.Wait()

// 					return nil

// 				}, nil, nil))
// 				errPanic(err)

// 				if !exists(c, b.name) {
// 					t.Errorf("Datastore %s: Blob does not exist yet", c.Kind())
// 				}

// 				wg2.Done()
// 			}(i)
// 		}

// 		wg2.Wait()
// 	})
// }

// // Invalid names behave just as if there was no blob with such name.
// // Writing such blob would always fail on close (similarly to how invalid name
// // when writing behaves)
// var invalidNames = []string{
// 	"",
// 	"short",
// 	"invalid-character",
// }

// func TestOpenInvalidName(t *testing.T) {
// 	allDS(func(c DS) {
// 		for _, n := range invalidNames {
// 			_, e := c.Open(n)
// 			if e != ErrNotFound {
// 				t.Fatalf("Datastore %s: Incorrect error for invalid name: %v", c.Kind(), e)
// 			}
// 		}
// 	})
// }

// func TestSaveInvalidName(t *testing.T) {
// 	allDS(func(c DS) {
// 		for _, n := range invalidNames {
// 			readCalled, eofCalledCnt, closeCalledCnt := false, 0, 0
// 			e := c.Save(n, bReader([]byte{}, func() error {
// 				if closeCalledCnt > 0 {
// 					t.Fatalf("Datastore %s: Read after Close()", c.Kind())
// 				}
// 				if eofCalledCnt > 0 {
// 					t.Fatalf("Datastore %s: Read after EOF", c.Kind())
// 				}
// 				readCalled = true
// 				return nil
// 			}, func() error {
// 				if closeCalledCnt > 0 {
// 					t.Fatalf("Datastore %s: EOF after Close()", c.Kind())
// 				}
// 				eofCalledCnt++
// 				return nil
// 			}, func() error {
// 				closeCalledCnt++
// 				return nil
// 			}))
// 			if e != ErrNameMismatch {
// 				t.Fatalf("Datastore %s: Got error when opening invalid name write stream: %v", c.Kind(), e)
// 			}
// 			if !readCalled {
// 				t.Fatalf("Datastore %s: Didn't call read for incorrect blob name", c.Kind())
// 			}
// 			if eofCalledCnt == 0 {
// 				t.Fatalf("Datastore %s: Didn't get EOF when reading incorrect blob data", c.Kind())
// 			}
// 			if eofCalledCnt > 1 {
// 				t.Fatalf("Datastore %s: Did get EOF multiple times (%d)", c.Kind(), eofCalledCnt)
// 			}
// 			if closeCalledCnt == 0 {
// 				t.Fatalf("Datastore %s: Didn't call close for incorrect blob name", c.Kind())
// 			}
// 			if closeCalledCnt > 1 {
// 				t.Fatalf("Datastore %s: Close called multiple times for incorrect blob name", c.Kind())
// 			}
// 		}
// 	})
// }

// func TestExistsInvalidName(t *testing.T) {
// 	allDS(func(c DS) {
// 		for _, n := range invalidNames {
// 			ex, err := c.Exists(n)
// 			errPanic(err)
// 			if ex {
// 				t.Fatalf("Datastore %s: Blob with invalid name exists", c.Kind())
// 			}
// 		}
// 	})
// }

// func TestDeleteInvalidNonExisting(t *testing.T) {
// 	allDS(func(c DS) {
// 		for _, b := range testBlobs {
// 			e := c.Delete(b.name)
// 			if e != ErrNotFound {
// 				t.Fatalf("Datastore %s: Incorrect error for invalid name: %v", c.Kind(), e)
// 			}
// 		}
// 	})
// }

// func TestDeleteInvalidName(t *testing.T) {
// 	allDS(func(c DS) {
// 		for _, n := range invalidNames {
// 			e := c.Delete(n)
// 			if e != ErrNotFound {
// 				t.Fatalf("Datastore %s: Incorrect error for invalid name: %v", c.Kind(), e)
// 			}
// 		}
// 	})
// }
