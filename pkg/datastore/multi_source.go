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

package datastore

import (
	"context"
	"io"
	"sync"
	"time"

	"golang.org/x/exp/slog"

	"github.com/cinode/go/pkg/common"
)

type multiSourceDatastoreBlobState struct {
	lastUpdateTime           time.Time
	downloading              bool
	downloadingFinishedCChan chan struct{}
}

type multiSourceDatastore struct {
	// Main datastore
	main DS

	// Additional sources that will be queried whenever the main source
	// does not contain the data or contains outdated content
	additional []DS

	// Average time between dynamic content refreshes
	dynamicDataRefreshTime time.Duration

	// Last update time for blobs, either for dynamic content or last result of not found for
	// static ones
	blobStates map[string]multiSourceDatastoreBlobState

	// Guard additional sources and update time map
	m sync.Mutex

	// Logger output
	log *slog.Logger
}

func NewMultiSource(main DS, refreshTime time.Duration, additional ...DS) DS {
	return &multiSourceDatastore{
		main:                   main,
		additional:             additional,
		dynamicDataRefreshTime: refreshTime,
		blobStates:             map[string]multiSourceDatastoreBlobState{},
		log:                    slog.Default(),
	}
}

var _ DS = (*multiSourceDatastore)(nil)

func (m *multiSourceDatastore) Kind() string {
	return "MultiSource"
}

func (m *multiSourceDatastore) Address() string {
	return "multi-source://"
}

func (m *multiSourceDatastore) Open(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	m.fetch(ctx, name)
	return m.main.Open(ctx, name)
}

func (m *multiSourceDatastore) Update(ctx context.Context, name *common.BlobName, r io.Reader) error {
	return m.main.Update(ctx, name, r)
}

func (m *multiSourceDatastore) Exists(ctx context.Context, name *common.BlobName) (bool, error) {
	m.fetch(ctx, name)
	return m.main.Exists(ctx, name)
}

func (m *multiSourceDatastore) Delete(ctx context.Context, name *common.BlobName) error {
	return m.main.Delete(ctx, name)
}

func (m *multiSourceDatastore) fetch(ctx context.Context, name *common.BlobName) {
	// TODO:
	// if not found locally, go over all additional sources and check if exists,
	// for dynamic content, perform merge operation if found in more than one,
	// initially in the background
	// also if dynamic content is queried and it was not updated in enough time,
	// do an update process

	for {
		waitChan, startDownload := func() (chan struct{}, bool) {
			m.m.Lock()
			defer m.m.Unlock()

			needsDownload := false

			state, found := m.blobStates[name.String()]

			switch {
			case !found:
				// Seen for the first time
				needsDownload = true

			case state.downloading:
				// Blob currently being downloaded
				return state.downloadingFinishedCChan, false

			case time.Since(state.lastUpdateTime) > m.dynamicDataRefreshTime:
				// We should update the blob
				needsDownload = true
			}

			if !needsDownload {
				return nil, false
			}

			// State not found, request new download
			ch := make(chan struct{})
			m.blobStates[name.String()] = multiSourceDatastoreBlobState{
				downloading:              true,
				downloadingFinishedCChan: ch,
			}
			return ch, true
		}()

		if startDownload {
			m.log.Info("Starting download",
				"blob", name.String(),
			)
			wasUpdated := false
			for i, ds := range m.additional {
				r, err := ds.Open(ctx, name)
				if err != nil {
					m.log.Debug("Failed to fetch blob from additional datastore",
						"blob", name.String(),
						"datastore", ds.Address(),
						"err", err,
					)
					continue
				}

				m.log.Info("Blob found in additional datastore",
					"blob", name.String(),
					"datastore-num", i+1,
				)
				err = m.main.Update(ctx, name, r)
				r.Close()
				if err != nil {
					m.log.Error("Failed to store blob in local datastore", err,
						"blob", name.String(),
					)
				}
				wasUpdated = true
			}
			if !wasUpdated {
				m.log.Warn("Did not find blob in any datastore",
					"blob", name.String(),
				)
			}
			defer close(waitChan)

			m.m.Lock()
			defer m.m.Unlock()

			m.blobStates[name.String()] = multiSourceDatastoreBlobState{
				lastUpdateTime: time.Now(),
			}
			return
		}

		if waitChan == nil {
			return
		}

		<-waitChan
	}
}
