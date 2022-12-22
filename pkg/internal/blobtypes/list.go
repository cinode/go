package blobtypes

import "github.com/cinode/go/pkg/common"

var (
	Invalid = common.NewBlobType(0x00)
	Static  = common.NewBlobType(0x01)
)

var All = map[string]common.BlobType{
	"Static": Static,
}
