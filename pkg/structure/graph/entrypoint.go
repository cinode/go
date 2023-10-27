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

package graph

import (
	"errors"
	"fmt"
	"time"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
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
	ep *protobuf.Entrypoint
	bn common.BlobName
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
	data := protobuf.Entrypoint{}

	err := proto.Unmarshal(b, &data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEntrypointDataParse, err)
	}

	return entrypointFromProtobuf(&data)
}

func entrypointFromProtobuf(data *protobuf.Entrypoint) (*Entrypoint, error) {
	if data == nil {
		return nil, ErrInvalidEntrypointDataNil
	}

	ret := Entrypoint{ep: data}

	// Extract blob name from entrypoint
	bn, err := common.BlobNameFromBytes(data.BlobName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEntrypointData, err)
	}
	ret.bn = bn

	// Links must not have mimetype set
	if ret.IsLink() && data.MimeType != "" {
		return nil, ErrInvalidEntrypointDataLinkMimetype
	}

	return &ret, nil
}

func EntrypointFromBlobNameAndKey(bn common.BlobName, key common.BlobKey) *Entrypoint {
	return entrypointFromBlobNameKeyAndProtoEntrypoint(bn, key, &protobuf.Entrypoint{})
}

func entrypointFromBlobNameKeyAndProtoEntrypoint(bn common.BlobName, key common.BlobKey, protoEp *protobuf.Entrypoint) *Entrypoint {
	protoEp.BlobName = bn.Bytes()
	protoEp.KeyInfo = &protobuf.KeyInfo{Key: key.Bytes()}

	return &Entrypoint{
		ep: protoEp,
		bn: bn,
	}
}

func (e *Entrypoint) String() string {
	return base58.Encode(e.Bytes())
}

func (e *Entrypoint) Bytes() []byte {
	return golang.Must(proto.Marshal(e.ep))
}

func (e *Entrypoint) BlobName() common.BlobName {
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

func (e *Entrypoint) IsValid(now time.Time) error {
	nowMicro := now.UnixMicro()
	if e.ep.NotValidBeforeUnixMicro != 0 {
		if e.ep.NotValidBeforeUnixMicro > nowMicro {
			return ErrNotYetValid
		}
	}
	if e.ep.NotValidAfterUnixMicro != 0 {
		if e.ep.NotValidAfterUnixMicro < nowMicro {
			return ErrExpired
		}
	}
	return nil
}
