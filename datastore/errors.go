package datastore

import "errors"

var (
	ErrUploadInProgress = errors.New("another upload is already in progress")
)
