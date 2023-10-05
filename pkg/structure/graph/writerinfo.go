package graph

import (
	"errors"
	"fmt"

	"github.com/cinode/go/pkg/structure/internal/protobuf"
	"github.com/jbenet/go-base58"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidWriterInfoData      = errors.New("invalid writer info data")
	ErrInvalidWriterInfoDataParse = fmt.Errorf("%w: protobuf parse error", ErrInvalidWriterInfoData)
)

type WriterInfo struct {
	wi *protobuf.WriterInfo
}

func WriterInfoFromString(s string) (WriterInfo, error) {
	if len(s) == 0 {
		return WriterInfo{}, fmt.Errorf("%w: empty string", ErrInvalidWriterInfoData)
	}

	b := base58.Decode(s)
	if len(b) == 0 {
		return WriterInfo{}, fmt.Errorf("%w: not a base58 string", ErrInvalidWriterInfoData)
	}

	return WriterInfoFromBytes(b)
}

func WriterInfoFromBytes(b []byte) (WriterInfo, error) {
	data := protobuf.WriterInfo{}

	err := proto.Unmarshal(b, &data)
	if err != nil {
		return WriterInfo{}, fmt.Errorf("%w: %s", ErrInvalidWriterInfoDataParse, err)
	}

	return writerInfoFromProtobuf(&data)
}

func writerInfoFromProtobuf(data *protobuf.WriterInfo) (WriterInfo, error) {
	return WriterInfo{wi: data}, nil
}
