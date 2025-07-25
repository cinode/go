/*
Copyright © 2025 Bartłomiej Święcki (byo)

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
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestDatastoreTestSuite(t *testing.T) {
	t.Run("InMemory", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			CreateDS: func() (DS, error) { return InMemory(), nil },
		})
	})

	t.Run("InFileSystem", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			CreateDS: func() (DS, error) { return InFileSystem(t.TempDir()) },
		})
	})

	t.Run("InRawFileSystem", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			CreateDS: func() (DS, error) { return InRawFileSystem(t.TempDir()) },
		})
	})

	t.Run("FromWeb", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			CreateDS: func() (DS, error) {
				server := httptest.NewServer(WebInterface(InMemory()))
				t.Cleanup(func() { server.Close() })

				return FromWeb(server.URL + "/")
			},
		})
	})
}
