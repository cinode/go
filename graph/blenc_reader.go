package graph

import (
	"bufio"
	"encoding/binary"
	"io"
)

func newBlencReader(r io.Reader) *blencReader {
	return &blencReader{r: bufio.NewReader(r), err: nil}
}

type blencReader struct {
	r   *bufio.Reader
	err error
}

func (s *blencReader) setErr(err error) error {
	if err != nil && s.err == nil {
		s.err = err
	}
	return s.err
}

func (s *blencReader) UInt() uint64 {
	if s.err != nil {
		return 0
	}

	ret, err := binary.ReadUvarint(s.r)
	if s.setErr(err) != nil {
		return 0
	}

	return ret
}

func (s *blencReader) Buff(b []byte) {
	if s.err == nil {
		_, err := s.r.Read(b)
		s.setErr(err)
	}
}

func (s *blencReader) String(maxLen uint64, maxLenErr error) string {
	l := s.UInt()
	if l > maxLen {
		s.setErr(maxLenErr)
		return ""
	}
	b := make([]byte, l)
	s.Buff(b)
	if s.err != nil {
		return ""
	}
	return string(b)
}
