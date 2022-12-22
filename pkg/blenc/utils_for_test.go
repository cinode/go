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

package blenc

// import (
// 	"bytes"
// 	"io"
// 	"strings"

// 	"github.com/cinode/go/pkg/datastore"
// )

// func errPanic(e error) {
// 	if e != nil {
// 		panic("Unexpected error: " + e.Error())
// 	}
// }

// type helperReader struct {
// 	buf     io.Reader
// 	onRead  func() error
// 	onEOF   func() error
// 	onClose func() error
// }

// func bReader(b []byte, onRead func() error, onEOF func() error, onClose func() error) *helperReader {

// 	nop := func() error {
// 		return nil
// 	}

// 	if onRead == nil {
// 		onRead = nop
// 	}
// 	if onEOF == nil {
// 		onEOF = nop
// 	}
// 	if onClose == nil {
// 		onClose = nop
// 	}

// 	return &helperReader{
// 		buf:     bytes.NewReader(b),
// 		onRead:  onRead,
// 		onEOF:   onEOF,
// 		onClose: onClose,
// 	}
// }

// func (h *helperReader) Read(b []byte) (n int, err error) {
// 	err = h.onRead()
// 	if err != nil {
// 		return 0, err
// 	}

// 	n, err = h.buf.Read(b)
// 	if err == io.EOF {
// 		err = h.onEOF()
// 		if err != nil {
// 			return 0, err
// 		}
// 		return 0, io.EOF
// 	}

// 	return n, err
// }

// func (h *helperReader) Close() error {
// 	return h.onClose()
// }

// func allBE(f func(be BE)) {
// 	func() {

// 		f(FromDatastore(datastore.InMemory()))

// 	}()
// }

// func allKG(f func(kg KeyDataGenerator)) {

// 	func() {
// 		// Test constant key generator
// 		f(constantKey([]byte(strings.Repeat("*", 32))))
// 	}()

// 	func() {
// 		// Test random key generator
// 		f(RandomKey())
// 	}()

// 	func() {
// 		// Test contents-based key generator
// 		f(ContentsHashKey())
// 	}()

// }

// func allBEKG(f func(be BE, kg KeyDataGenerator)) {
// 	allBE(func(be BE) {
// 		allKG(func(kg KeyDataGenerator) {
// 			f(be, kg)
// 		})
// 	})
// }

// func beSave(be BE, data string) (name string, key string) {
// 	kg := constantKey([]byte(strings.Repeat("*", 32)))
// 	name, key, err := be.Save(bReader([]byte(data), nil, nil, nil), kg)
// 	errPanic(err)
// 	return name, key
// }

// func beExists(be BE, name string) bool {
// 	ret, err := be.Exists(name)
// 	errPanic(err)
// 	return ret
// }
