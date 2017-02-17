package graph

import (
	"bufio"
	"encoding/binary"
	"io"
)

func newBeReader(r io.Reader) *beReader {
	return &beReader{r: bufio.NewReader(r), err: nil}
}

type beReader struct {
	r   *bufio.Reader
	err error
}

func (s *beReader) setErr(err error) bool {
	if err != nil && s.err == nil {
		s.err = err
	}
	return s.err == nil
}

func (s *beReader) UInt() uint64 {
	if s.err != nil {
		return 0
	}

	ret, err := binary.ReadUvarint(s.r)
	if !s.setErr(err) {
		return 0
	}

	return ret
}

func (s *beReader) Buff(b []byte) {
	_, err := s.r.Read(b)
	s.setErr(err)
}

func (s *beReader) String(maxLen uint64) string {
	l := s.UInt()
	if l > maxLen {
		s.setErr(ErrMalformedDirectoryBlob)
		return ""
	}
	b := make([]byte, l)
	s.Buff(b)
	if s.err != nil {
		return ""
	}
	return string(b)
}
