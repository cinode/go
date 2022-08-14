package datastore

import "errors"

var (
	ErrValidationFailed = errors.New("blob validation failed")
	ErrUploadInProgress = errors.New("another upload is already in progress")
)
