/*
Copyright © 2023 Bartłomiej Święcki (byo)

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

package dynamiclink

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestVectors(t *testing.T) {
	err := filepath.WalkDir("../../../../testvectors/dynamic", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		testCase := struct {
			Name             string   `json:"name"`
			Description      string   `json:"description"`
			Details          []string `json:"details"`
			BlobName         []byte   `json:"blob_name"`
			EncryptionKey    []byte   `json:"encryption_key"`
			UpdateDataset    []byte   `json:"update_dataset"`
			DecryptedDataset []byte   `json:"decrypted_dataset"`
			ValidPublicly    bool     `json:"valid_publicly"`
			ValidPrivately   bool     `json:"valid_privately"`
			GoErrorContains  string   `json:"go_error_contains"`
		}{}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		err = json.Unmarshal(data, &testCase)
		if err != nil {
			return err
		}

		det := strings.Join(testCase.Details, "\n")

		t.Run(testCase.Name, func(t *testing.T) {
			t.Run("validate public scope", func(t *testing.T) {
				err := func() error {
					pr, err := FromPublicData(
						common.BlobName(testCase.BlobName),
						bytes.NewReader(testCase.UpdateDataset),
					)
					if err != nil {
						return err
					}

					dr := pr.GetEncryptedLinkReader()

					_, err = io.ReadAll(dr)
					if err != nil {
						return err
					}

					return nil
				}()

				if testCase.ValidPublicly {
					require.NoError(t, err, det)
				} else {
					require.ErrorContains(t, err, testCase.GoErrorContains, det)
				}
			})

			t.Run("validate private scope", func(t *testing.T) {
				err := func() error {

					pr, err := FromPublicData(
						common.BlobName(testCase.BlobName),
						bytes.NewReader(testCase.UpdateDataset),
					)
					if err != nil {
						return err
					}

					dr, err := pr.GetLinkDataReader(common.BlobKey(testCase.EncryptionKey))
					if err != nil {
						return err
					}

					data, err := io.ReadAll(dr)
					if err != nil {
						return err
					}

					// If we've got here - validation passed, the dataset must be correct.
					// If it is not it may indicate failure to detect attack
					require.Equal(t, testCase.DecryptedDataset, data)
					return nil
				}()

				if testCase.ValidPrivately {
					require.NoError(t, err, det)
				} else {
					require.ErrorContains(t, err, testCase.GoErrorContains, det)
				}
			})
		})

		return nil
	})
	require.NoError(t, err)
}
