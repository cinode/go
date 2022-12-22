package blobtypes

import "errors"

var (
	ErrUnknownBlobType  = errors.New("unknown blob type")
	ErrValidationFailed = errors.New("blob validation failed")
)
