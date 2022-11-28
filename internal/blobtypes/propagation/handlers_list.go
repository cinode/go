package propagation

import (
	"fmt"

	"github.com/cinode/go/common"
	"github.com/cinode/go/internal/blobtypes"
)

func HandlerForType(t common.BlobType) (Handler, error) {
	switch t {
	case blobtypes.Static:
		return newStaticBlobHandlerSha256(), nil
	}

	return nil, fmt.Errorf("%w (%d)", blobtypes.ErrUnknownBlobType, t)
}
