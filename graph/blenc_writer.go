package graph

import (
	"encoding/binary"
	"io"
)

func newBlencWriter(w io.Writer) *blencWriter {
	return &blencWriter{w: w, err: nil}
}

type blencWriter struct {
	w   io.Writer
	err error
}

func (s *blencWriter) setErr(err error) bool {
	if err != nil && s.err == nil {
		s.err = err
	}
	return s.err != nil
}

func (s *blencWriter) UInt(i uint64) {
	buff := make([]byte, 16)
	n := binary.PutUvarint(buff, i)
	s.Buff(buff[:n])
}

func (s *blencWriter) Buff(b []byte) {
	_, err := s.w.Write(b)
	s.setErr(err)
}

func (s *blencWriter) String(str string) {
	b := []byte(str)
	s.UInt(uint64(len(b)))
	s.Buff(b)
}
