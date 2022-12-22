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

package datastore

import (
	"errors"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
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
