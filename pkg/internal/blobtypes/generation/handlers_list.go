package generation

import (
	"fmt"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
)

func HandlerForType(t common.BlobType) (Handler, error) {
	switch t {
	case blobtypes.Static:
		return newStaticBlobHandlerSha256(), nil
	}

	return nil, fmt.Errorf("%w (%d)", blobtypes.ErrUnknownBlobType, t)
}
