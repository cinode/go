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

package multisource

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
)

func TestOptions(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	additionalDatastores := []datastore.DS{
		datastore.InMemory(),
		datastore.InMemory(),
		datastore.InMemory(),
	}

	ds := New(
		datastore.InMemory(),
		WithDynamicDataRefreshTime(7*time.Second),
		WithNotFoundRecheckTime(13*time.Second),
		WithLogger(log),
		WithAdditionalDatastores(additionalDatastores...),
	).(*multiSourceDatastore)

	require.Equal(t, 7*time.Second, ds.dynamicDataRefreshTime)
	require.Equal(t, 13*time.Second, ds.notFoundRecheckTime)
	require.Equal(t, log, ds.log)
	require.Equal(t, additionalDatastores, ds.additional)
}
