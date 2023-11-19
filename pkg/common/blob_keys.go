/*
Copyright © 2023 Bartłomiej Święcki (byo)

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

package common

import "crypto/subtle"

func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	ret := make([]byte, len(b))
	copy(ret, b)
	return ret
}

// Key with cipher type
type BlobKey struct{ key []byte }

func BlobKeyFromBytes(key []byte) *BlobKey { return &BlobKey{key: copyBytes(key)} }
func (k *BlobKey) Bytes() []byte           { return copyBytes(k.key) }
func (k *BlobKey) Equal(k2 *BlobKey) bool  { return subtle.ConstantTimeCompare(k.key, k2.key) == 1 }

// IV
type BlobIV struct{ iv []byte }

func BlobIVFromBytes(iv []byte) *BlobIV { return &BlobIV{iv: copyBytes(iv)} }
func (i *BlobIV) Bytes() []byte         { return copyBytes(i.iv) }
func (i *BlobIV) Equal(i2 *BlobIV) bool { return subtle.ConstantTimeCompare(i.iv, i2.iv) == 1 }
