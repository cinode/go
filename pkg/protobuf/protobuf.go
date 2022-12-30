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

package protobuf

import (
	"google.golang.org/protobuf/proto"
)

//go:generate protoc --go_out=. protobuf.proto

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
