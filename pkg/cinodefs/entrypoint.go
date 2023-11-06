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

package cinodefs

import (
	"errors"
	"fmt"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/cinodefs/internal/protobuf"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidEntrypointData             = errors.New("invalid entrypoint data")
	ErrInvalidEntrypointDataParse        = fmt.Errorf("%w: protobuf parse error", ErrInvalidEntrypointData)
	ErrInvalidEntrypointDataLinkMimetype = fmt.Errorf("%w: link can not have mimetype set", ErrInvalidEntrypointData)
	ErrInvalidEntrypointDataNil          = fmt.Errorf("%w: nil data", ErrInvalidEntrypointData)
	ErrInvalidEntrypointTime             = errors.New("time validation failed")
	ErrExpired                           = fmt.Errorf("%w: entry expired", ErrInvalidEntrypointTime)
	ErrNotYetValid                       = fmt.Errorf("%w: entry not yet valid", ErrInvalidEntrypointTime)
)

type Entrypoint struct {
	ep protobuf.Entrypoint
	bn *common.BlobName
}

func EntrypointFromString(s string) (*Entrypoint, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("%w: empty string", ErrInvalidEntrypointData)
	}

	b := base58.Decode(s)
	if len(b) == 0 {
		return nil, fmt.Errorf("%w: not a base58 string", ErrInvalidEntrypointData)
	}

	return EntrypointFromBytes(b)
}

func EntrypointFromBytes(b []byte) (*Entrypoint, error) {
	ep := &Entrypoint{}

	err := proto.Unmarshal(b, &ep.ep)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEntrypointDataParse, err)
	}

	err = expandEntrypointProto(ep)
	if err != nil {
		return nil, err
	}

	return ep, nil
}

func entrypointFromProtobuf(data *protobuf.Entrypoint) (*Entrypoint, error) {
	if data == nil {
		return nil, ErrInvalidEntrypointDataNil
	}

	ep := &Entrypoint{}
	proto.Merge(&ep.ep, data)
	err := expandEntrypointProto(ep)
	if err != nil {
		return nil, err
	}
	return ep, nil
}

func expandEntrypointProto(ep *Entrypoint) error {
	// Extract blob name from entrypoint
	bn, err := common.BlobNameFromBytes(ep.ep.BlobName)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidEntrypointData, err)
	}
	ep.bn = bn

	// Links must not have mimetype set
	if ep.IsLink() && ep.ep.MimeType != "" {
		return ErrInvalidEntrypointDataLinkMimetype
	}

	return nil
}

func EntrypointFromBlobNameAndKey(bn *common.BlobName, key *common.BlobKey) *Entrypoint {
	return setEntrypointBlobNameAndKey(bn, key, &Entrypoint{})
}

func setEntrypointBlobNameAndKey(bn *common.BlobName, key *common.BlobKey, ep *Entrypoint) *Entrypoint {
	ep.bn = bn
	ep.ep.BlobName = bn.Bytes()
	ep.ep.KeyInfo = &protobuf.KeyInfo{Key: key.Bytes()}
	return ep
}

func (e *Entrypoint) String() string {
	return base58.Encode(e.Bytes())
}

func (e *Entrypoint) Bytes() []byte {
	return golang.Must(proto.Marshal(&e.ep))
}

func (e *Entrypoint) BlobName() *common.BlobName {
	return e.bn
}

func (e *Entrypoint) IsLink() bool {
	return e.bn.Type() == blobtypes.DynamicLink
}

func (e *Entrypoint) IsDir() bool {
	return e.ep.MimeType == CinodeDirMimeType
}

func (e *Entrypoint) MimeType() string {
	return e.ep.MimeType
}
