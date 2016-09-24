package datastore

import "crypto/subtle"

func nameCheckForSave(expectedName string) func(string) bool {
	return func(producedName string) bool {
		return subtle.ConstantTimeCompare(
			[]byte(expectedName),
			[]byte(producedName),
		) == 1
	}
}
