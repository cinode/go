package graph

import (
	"encoding/binary"
	"io"
)

func newBeWriter(w io.Writer) *beWriter {
	return &beWriter{w: w, err: nil}
}

type beWriter struct {
	w   io.Writer
	err error
}

func (s *beWriter) setErr(err error) bool {
	if err != nil && s.err == nil {
		s.err = err
	}
	return s.err != nil
}

func (s *beWriter) UInt(i uint64) {
	buff := make([]byte, 16)
	n := binary.PutUvarint(buff, i)
	s.Buff(buff[:n])
}

func (s *beWriter) Buff(b []byte) {
	_, err := s.w.Write(b)
	s.setErr(err)
}

func (s *beWriter) String(str string) {
	b := []byte(str)
	s.UInt(uint64(len(b)))
	s.Buff(b)
}
