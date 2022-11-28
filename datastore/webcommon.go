package datastore

import (
	"errors"

	"github.com/cinode/go/common"
	"github.com/cinode/go/internal/blobtypes"
)

type webErrResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	webErrMap = map[string]error{
		"UNKNOWN_BLOB_TYPE":  blobtypes.ErrUnknownBlobType,
		"VALIDATION_FAILED":  blobtypes.ErrValidationFailed,
		"INVALID_BLOB_NAME":  common.ErrInvalidBlobName,
		"UPLOAD_IN_PROGRESS": ErrUploadInProgress,
		"NO_FORM_FIELD":      errNoData,
	}
)

type webNameResponse struct {
	Name string `json:"name"`
}

func webErrToCode(err error) string {
	for code, errMatch := range webErrMap {
		if errors.Is(err, errMatch) {
			return code
		}
	}
	return ""
}

func webErrFromCode(code string) error {
	if err, ok := webErrMap[code]; ok {
		return err
	}
	return nil
}
