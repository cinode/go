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

package protobuf

//go:generate protoc --go_out=. protobuf.proto

import (
	"errors"
	"time"

	"github.com/cinode/go/pkg/common"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidEntrypoint     = errors.New("invalid entrypoint")
	ErrInvalidEntrypointTime = errors.New("%w: time validation failed")
)

func (ep *Entrypoint) ToBytes() ([]byte, error) {
	return proto.Marshal(ep)
}

func EntryPointFromBytes(b []byte) (*Entrypoint, error) {
	ret := &Entrypoint{}
	err := proto.Unmarshal(b, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (wi *WriterInfo) ToBytes() ([]byte, error) {
	return proto.Marshal(wi)
}

func WriterInfoFromBytes(b []byte) (*WriterInfo, error) {
	ret := &WriterInfo{}
	err := proto.Unmarshal(b, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (ep *Entrypoint) Validate(currentTime time.Time) error {
	currentTimeMicro := currentTime.UnixMicro()

	if ep.GetNotValidAfterUnixMicro() != 0 &&
		currentTimeMicro > ep.GetNotValidAfterUnixMicro() {
		return ErrInvalidEntrypointTime
	}

	if ep.GetNotValidBeforeUnixMicro() != 0 &&
		currentTimeMicro < ep.GetNotValidBeforeUnixMicro() {
		return ErrInvalidEntrypointTime
	}

	return nil
}

func (ep *Entrypoint) ValidateAndParse(currentTime time.Time) (
	*common.BlobName,
	*common.BlobKey,
	error,
) {
	err := ep.Validate(currentTime)
	if err != nil {
		return nil, nil, err
	}

	bn, err := common.BlobNameFromBytes(ep.BlobName)
	if err != nil {
		return nil, nil, err
	}

	key := common.BlobKeyFromBytes(ep.GetKeyInfo().GetKey())

	return bn, key, nil
}
