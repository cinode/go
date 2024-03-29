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

package testblobs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/common"
)

type TestBlob struct {
	UpdateDataset    []byte
	BlobName         *common.BlobName
	EncryptionKey    *common.BlobKey
	DecryptedDataset []byte
}

func (s *TestBlob) Put(baseUrl string) error {
	return s.PutWithAuth(baseUrl, "", "")
}

func (s *TestBlob) PutWithAuth(baseUrl, username, password string) error {
	finalUrl, err := url.JoinPath(baseUrl, s.BlobName.String())
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPut,
		finalUrl,
		bytes.NewReader(s.UpdateDataset))
	if err != nil {
		return err
	}

	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("invalid http status code %s (%d), body: %s", resp.Status, resp.StatusCode, string(body))
	}

	return nil
}

func (s *TestBlob) Get(baseUrl string) ([]byte, error) {
	finalUrl, err := url.JoinPath(baseUrl, s.BlobName.String())
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(finalUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("invalid http status code %s (%d), body: %s", resp.Status, resp.StatusCode, string(body))
	}

	return body, nil
}

func (s *TestBlob) Entrypoint() *cinodefs.Entrypoint {
	return cinodefs.EntrypointFromBlobNameAndKey(
		s.BlobName,
		s.EncryptionKey,
	)
}
