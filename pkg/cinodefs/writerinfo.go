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

	"github.com/cinode/go/pkg/cinodefs/protobuf"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidWriterInfoData      = errors.New("invalid writer info data")
	ErrInvalidWriterInfoDataParse = fmt.Errorf("%w: protobuf parse error", ErrInvalidWriterInfoData)
)

type WriterInfo struct {
	wi protobuf.WriterInfo
}

func (wi *WriterInfo) Bytes() []byte {
	return golang.Must(proto.Marshal(&wi.wi))
}

func (wi *WriterInfo) String() string {
	return base58.Encode(wi.Bytes())
}

func WriterInfoFromString(s string) (*WriterInfo, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("%w: empty string", ErrInvalidWriterInfoData)
	}

	b := base58.Decode(s)
	if len(b) == 0 {
		return nil, fmt.Errorf("%w: not a base58 string", ErrInvalidWriterInfoData)
	}

	return WriterInfoFromBytes(b)
}

func WriterInfoFromBytes(b []byte) (*WriterInfo, error) {
	wi := WriterInfo{}

	err := proto.Unmarshal(b, &wi.wi)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidWriterInfoDataParse, err)
	}

	return &wi, nil
}

func writerInfoFromBlobNameKeyAndAuthInfo(bn *common.BlobName, key *common.BlobKey, authInfo *common.AuthInfo) *WriterInfo {
	return &WriterInfo{
		wi: protobuf.WriterInfo{
			BlobName: bn.Bytes(),
			Key:      key.Bytes(),
			AuthInfo: authInfo.Bytes(),
		},
	}
}
