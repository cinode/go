/*
Copyright © 2025 Bartłomiej Święcki (byo)

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

package validatingreader

import "io"

type onEOFCheckReader struct {
	r            io.Reader
	onCloseCheck func() error
}

func (h onEOFCheckReader) Read(b []byte) (int, error) {
	n, err := h.r.Read(b)

	if err == io.EOF {
		err2 := h.onCloseCheck()
		if err2 != nil {
			return n, err2
		}
	}

	return n, err
}

func CheckOnEOF(r io.Reader, check func() error) io.Reader {
	return onEOFCheckReader{
		r:            r,
		onCloseCheck: check,
	}
}
