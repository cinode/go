package datastore

import "errors"

type webErrResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	webErrMap = map[string]error{
		"UNKNOWN_BLOB_TYPE": ErrUnknownBlobType,
		"VALIDATION_FAILED": ErrValidationFailed,
		"INVALID_BLOB_NAME": ErrInvalidBlobName,
		"NO_FORM_FIELD":     errNoData,
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
