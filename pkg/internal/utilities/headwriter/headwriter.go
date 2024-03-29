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

package headwriter

type Writer struct {
	limit int
	data  []byte
}

func New(limit int) Writer {
	return Writer{
		limit: limit,
		data:  make([]byte, 0, limit),
	}
}

func (h *Writer) Write(b []byte) (int, error) {
	if len(h.data) >= h.limit {
		return len(b), nil
	}

	if len(h.data)+len(b) > h.limit {
		h.data = append(h.data, b[:h.limit-len(h.data)]...)
		return len(b), nil
	}

	h.data = append(h.data, b...)
	return len(b), nil
}

func (h *Writer) Head() []byte { return h.data }
