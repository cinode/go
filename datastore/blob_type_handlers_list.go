package datastore

import (
	"errors"
	"fmt"
)

var handlers = []BlobTypeHandler{
	NewStaticBlobHandlerSha256(),
}

var (
	ErrUnknownBlobType = errors.New("unknown blob type")
)

func handlerForType(t BlobType) (BlobTypeHandler, error) {
	for _, h := range handlers {
		if h.Type() == t {
			return h, nil
		}
	}

	return nil, fmt.Errorf("%w (%d)", ErrUnknownBlobType, t)
}
