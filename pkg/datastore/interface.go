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
	"context"
	"errors"
	"io"

	"github.com/cinode/go/pkg/common"
)

var (
	// ErrNotFound will be used when blob with given name was not found in datastore
	ErrNotFound = errors.New("not found")
)

// DS interface contains the public interface of any conformant datastore
//
// Stored data is split into small chunks called blobs. Each blob has its
// unique name. The name is used to perform cryptographic verification
// of the blob data (e.g. it must match the digest of data for static blobs).
//
// The blob content (in case of blobs other than the static blob) can be updated
// over time. The rule of forward progress is that if there are two or more
// valid datasets for a single blob, performing update of that blob
// with those datasets must be deterministic and always result with a single
// final dataset. The merge result may be the content one of the source datasets
// deterministically selected or a combined dataset containing information from
// other datasets merged.
//
// On the interface level, there is no distinction between blob types and
// their internal data. Working with that interface allows treating the dataset
// as completely opaque byte streams. This simplifies implementation of
// data through various transfer mechanisms independently from blob types.gaze
type DS interface {

	// Kind returns string representation of datastore kind (i.e. "Memory")
	Kind() string

	// Open returns a read stream for given blob name or an error. In case blob
	// is not found in datastore, returned error must be of ErrNotFound type.
	//
	// The blob may be detected to be invalid (not passing the validation),
	// in that case, either the Open call or the Read method from the returned
	// reader will return ErrInvalidData error.
	//
	// If a non-nil error is returned, the writer will be nil. Otherwise it
	// is necessary to call the `Close` on the returned reader once done
	// with the reader.
	Open(ctx context.Context, name common.BlobName) (io.ReadCloser, error)

	// Update retrieves an update for given blob. The data is read from given
	// reader until it returns either EOF, ending successful save, or any other
	// error which will cancel the save - in such case this error will be
	// returned from this function. If the data does not pass validation,
	// ErrInvalidData will be returned.
	Update(ctx context.Context, name common.BlobName, r io.Reader) error

	// Exists does check whether blob of given name exists in the datastore.
	// Partially written blobs are equal to non-existing ones. Boolean value
	// returned indicates whether the blob exists or not, non-nil error indicates
	// that there was an error while trying to check blob's existence.
	Exists(ctx context.Context, name common.BlobName) (bool, error)

	// Delete tries to remove blob with given name from the datastore.
	// If blob does not exist (which includes partially written blobs)
	// ErrNotFound will be returned. If blob is being opened at the moment
	// of removal, all opened references to the blob must still be able to
	// read the blob data. After the `Delete` call succeeds, trying to read
	// the blob with the `Open` should end up with an ErrNotFound error
	// until the blob is updated again with a successful `Update` call.
	Delete(ctx context.Context, name common.BlobName) error
}
