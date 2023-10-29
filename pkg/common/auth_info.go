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

// AuthInfo is an opaque data that is necessary to perform update of an existing blob.
//
// Currently used only for dynamic links, auth info contains all the necessary information
// to update the content of the blob. The representation is specific to the blob type
type AuthInfo struct{ data []byte }

func AuthInfoFromBytes(iv []byte) *AuthInfo { return &AuthInfo{data: copyBytes(iv)} }
func (a *AuthInfo) Bytes() []byte           { return copyBytes(a.data) }
func (a *AuthInfo) Equal(a2 *AuthInfo) bool { return subtle.ConstantTimeCompare(a.data, a2.data) == 1 }
