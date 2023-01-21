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

package dynamiclink

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func panicIf(b bool, msg interface{}) {
	if b {
		panic(fmt.Sprint(msg))
	}
}

func readBuff(r io.Reader, buff []byte, n string) error {
	_, err := io.ReadFull(r, buff)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%w while reading %s", ErrInvalidDynamicLinkDataTruncated, n)
	}
	if err != nil {
		return err
	}
	return nil
}

func readDynamicSizeBuff(r io.Reader, n string) ([]byte, error) {
	size, err := readByte(r, n)
	if err != nil {
		return nil, err
	}
	if size >= 0x80 {
		return nil, fmt.Errorf("%w while reading %s", ErrInvalidDynamicLinkDataBlockSize, n)
	}
	ret := make([]byte, size)
	err = readBuff(r, ret, n)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func readByte(r io.Reader, n string) (byte, error) {
	var b [1]byte
	err := readBuff(r, b[:], n)
	return b[0], err
}

func readUint64(r io.Reader, n string) (uint64, error) {
	var b [8]byte
	err := readBuff(r, b[:], n)
	return binary.BigEndian.Uint64(b[:]), err
}

// note: below raises errors through panics,
// that's because it is assumed to write to byte buffers
// and hashers that should not return an error

func storeBuff(w io.Writer, b []byte) {
	c, err := w.Write(b)
	panicIf(err != nil, err)
	panicIf(c != len(b), io.ErrShortWrite)
}

func storeDynamicSizeBuff(w io.Writer, b []byte) {
	panicIf(len(b) >= 0x80, "Block size too large")

	storeByte(w, byte(len(b)))
	storeBuff(w, b)
}

func storeByte(w io.Writer, b byte) {
	storeBuff(w, []byte{b})
}

func storeUint64(w io.Writer, v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	storeBuff(w, b[:])
}
