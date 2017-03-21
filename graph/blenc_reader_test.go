package graph

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestBlencReaderDeserializeVaules(t *testing.T) {

	for _, d := range []struct {
		b []byte
		v interface{}
	}{
		{[]byte{0x00}, uint64(0x00)},
		{[]byte{0x01}, uint64(0x01)},
		{[]byte{0x7F}, uint64(0x7F)},
		{[]byte{0x80, 0x01}, uint64(0x80)},
		{[]byte{0x00}, ""},
		{[]byte{0x01, 0x61}, "a"},
		{[]byte{0x03, 0x63, 0x62, 0x61}, "cba"},
	} {
		b := bytes.NewReader(d.b)
		r := newBlencReader(b)

		switch v := d.v.(type) {
		case uint64:
			read := r.UInt()
			if read != v {
				t.Fatalf("Incorrect value read, was %v, should be %v", read, v)
			}
		case string:
			read := r.String(0xFFF, nil)
			if read != v {
				t.Fatalf("Incorrect value read, was %v, should be %v", read, v)
			}
		default:
			panic("Invalid test - unknown type")
		}

		if r.err != nil {
			t.Fatalf("Unexpected error: %v", r.err)
		}
		_, err := b.ReadByte()
		if err != io.EOF {
			t.Fatalf("Extra bytes left at the end of stream (%v)", err)
		}
	}
}

func TestBlencReaderStringLimit(t *testing.T) {
	for _, d := range []struct {
		limit uint64
		err   bool
	}{
		{0, true},
		{1, true},
		{7, true},
		{8, false},
		{9, false},
		{100000, false},
	} {

		r := newBlencReader(bytes.NewReader([]byte{
			0x08, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
		}))

		errLimit := errors.New("test")

		r.String(d.limit, errLimit)
		if d.err {
			if r.err == nil {
				t.Fatal("String length limit did not apply")
			}
			if r.err != errLimit {
				t.Fatalf("Invalid limit error caught: %v", r.err)
			}
		} else {
			if r.err != nil {
				t.Fatalf("Couldn't read string of valid length (%v)", r.err)
			}
		}
	}
}

func TestBlencReaderSaneErrorValues(t *testing.T) {
	r := newBlencReader(bytes.NewReader([]byte{}))
	r.setErr(errors.New("test"))

	s := r.String(0x1000, nil)
	if s != "" {
		t.Fatalf("Invalid string on error returned: '%v'", s)
	}

	u := r.UInt()
	if u != 0 {
		t.Fatalf("Invalid uint64 on error returned: %v", u)
	}
}

func TestBlencReaderStopReadingOnError(t *testing.T) {
	br := bytes.NewReader([]byte{
		0x01, 0x02, 0x03, 0x04,
	})
	r := newBlencReader(br)
	r.setErr(errors.New("test"))

	if br.Len() != 4 {
		t.Fatalf("Incorrect number of bytes left in reader: %v", br.Len())
	}

	r.String(0x1000, nil)

	if br.Len() != 4 {
		t.Fatalf("Incorrect number of bytes left in reader: %v", br.Len())
	}

	r.UInt()

	if br.Len() != 4 {
		t.Fatalf("Incorrect number of bytes left in reader: %v", br.Len())
	}

	r.Buff(make([]byte, 10))

	if br.Len() != 4 {
		t.Fatalf("Incorrect number of bytes left in reader: %v", br.Len())
	}

}
