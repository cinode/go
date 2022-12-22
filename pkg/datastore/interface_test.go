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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
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
			err := ds.Read(context.Background(), emptyBlobName, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestOpenInvalidBlobType(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			bn, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), common.NewBlobType(0xFF))
			require.NoError(t, err)

			err = ds.Read(context.Background(), bn, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)

			err = ds.Update(context.Background(), bn, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)
		})
	}
}

func TestSaveNameMismatch(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			err := ds.Update(context.Background(), emptyBlobName, bytes.NewReader([]byte("test")))
			require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
		})
	}
}

func TestSaveSuccessful(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {

			for _, b := range testBlobs {

				exists, err := ds.Exists(context.Background(), b.name)
				require.NoError(t, err)
				require.False(t, exists)

				err = ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
				require.NoError(t, err)

				exists, err = ds.Exists(context.Background(), b.name)
				require.NoError(t, err)
				require.True(t, exists)

				// Overwrite with the same data must be fine
				err = ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
				require.NoError(t, err)

				exists, err = ds.Exists(context.Background(), b.name)
				require.NoError(t, err)
				require.True(t, exists)

				// Overwrite with wrong data must fail
				err = ds.Update(context.Background(), b.name, bytes.NewReader(append([]byte{0x00}, b.data...)))
				require.ErrorIs(t, err, blobtypes.ErrValidationFailed)

				exists, err = ds.Exists(context.Background(), b.name)
				require.NoError(t, err)
				require.True(t, exists)

				data := bytes.NewBuffer([]byte{})
				err = ds.Read(context.Background(), b.name, data)
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
				err := ds.Update(context.Background(), b.name, bReader(b.data, func() error {
					return errRet
				}, nil))
				require.ErrorIs(t, err, errRet)

				exists, err := ds.Exists(context.Background(), b.name)
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

			err := ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
			require.NoError(t, err)

			errRet := errors.New("cancel")

			err = ds.Update(context.Background(), b.name, bReader(b.data, func() error {
				exists, err := ds.Exists(context.Background(), b.name)
				require.NoError(t, err)
				require.True(t, exists)

				return errRet
			}, nil))

			require.ErrorIs(t, err, errRet)

			exists, err := ds.Exists(context.Background(), b.name)
			require.NoError(t, err)
			require.True(t, exists)

			data := bytes.NewBuffer(nil)
			err = ds.Read(context.Background(), b.name, data)
			require.NoError(t, err)
			require.Equal(t, b.data, data.Bytes())
		})
	}
}

func TestDeleteNonExisting(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			b := testBlobs[0]

			err := ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
			require.NoError(t, err)

			err = ds.Delete(context.Background(), testBlobs[1].name)
			require.ErrorIs(t, err, ErrNotFound)

			exists, err := ds.Exists(context.Background(), b.name)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}

func TestDeleteExisting(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {

			b := testBlobs[0]
			err := ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
			require.NoError(t, err)

			exists, err := ds.Exists(context.Background(), b.name)
			require.NoError(t, err)
			require.True(t, exists)

			err = ds.Delete(context.Background(), b.name)
			require.NoError(t, err)

			exists, err = ds.Exists(context.Background(), b.name)
			require.NoError(t, err)
			require.False(t, exists)

			err = ds.Read(context.Background(), b.name, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestGetKind(t *testing.T) {
	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {
			k := ds.Kind()
			require.NotEmpty(t, k)
		})
	}
}

func TestSimultaneousReads(t *testing.T) {
	const threadCnt = 10
	const readCnt = 200

	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {

			// Prepare data
			for _, b := range testBlobs {
				err := ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
				require.NoError(t, err)
			}

			wg := sync.WaitGroup{}
			wg.Add(threadCnt)

			for i := 0; i < threadCnt; i++ {
				go func(i int) {
					defer wg.Done()
					for n := 0; n < readCnt; n++ {
						b := testBlobs[(i+n)%len(testBlobs)]

						buff := bytes.NewBuffer([]byte{})
						err := ds.Read(context.Background(), b.name, buff)
						require.NoError(t, err)
						require.Equal(t, b.data, buff.Bytes())
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

func TestSimultaneousSaves(t *testing.T) {
	threadCnt := 3

	for _, ds := range allTestInterfaces(t) {
		t.Run(ds.Kind(), func(t *testing.T) {

			b := testBlobs[0]

			wg := sync.WaitGroup{}
			wg.Add(threadCnt)

			for i := 0; i < threadCnt; i++ {
				go func(i int) {
					defer wg.Done()

					err := ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
					if errors.Is(err, ErrUploadInProgress) {
						// TODO: We should be able to handle this case
						return
					}

					require.NoError(t, err)

					exists, err := ds.Exists(context.Background(), b.name)
					require.NoError(t, err)
					require.True(t, exists)
				}(i)
			}

			wg.Wait()

			exists, err := ds.Exists(context.Background(), b.name)
			require.NoError(t, err)
			require.True(t, exists)

			buf := bytes.NewBuffer([]byte{})
			err = ds.Read(context.Background(), b.name, buf)
			require.NoError(t, err)
			require.Equal(t, b.data, buf.Bytes())
		})
	}
}

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
