package graph

import (
	"bytes"
	"errors"
	"testing"
)

func TestBlencWriterSerializeValues(t *testing.T) {
	for _, d := range []struct {
		v interface{}
		b []byte
	}{
		{uint64(0x00), []byte{0x00}},
		{uint64(0x01), []byte{0x01}},
		{uint64(0x7F), []byte{0x7F}},
		{uint64(0x80), []byte{0x80, 0x01}},
		{"", []byte{0x00}},
		{"a", []byte{0x01, 0x61}},
		{"cba", []byte{0x03, 0x63, 0x62, 0x61}},
	} {
		b := bytes.NewBuffer(nil)
		w := newBlencWriter(b)
		switch v := d.v.(type) {
		case uint64:
			w.UInt(v)
		case string:
			w.String(v)
		default:
			panic("Invalid test case - unknown type")
		}
		if w.err != nil {
			t.Fatalf("Couldn't write integer: %v", w.err)
		}
		if !bytes.Equal(d.b, b.Bytes()) {
			t.Fatalf("Invalid byte representation")
		}
	}
}

func TestBlencWriterSetError(t *testing.T) {
	b := bytes.NewBuffer(nil)
	w := newBlencWriter(b)
	w.UInt(0)
	if len(b.Bytes()) != 1 {
		t.Fatal("Invalid bytes count")
	}
	if w.err != nil {
		t.Fatalf("Unwanted error: %v", w.err)
	}
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	w.setErr(err1)
	if w.err != err1 {
		t.Fatalf("Incorrect error caught: %v", w.err)
	}

	w.setErr(err2)
	if w.err != err1 {
		t.Fatalf("Incorrect error set (should remain the same): %v", w.err)
	}

	w.UInt(1)
	if len(b.Bytes()) != 1 {
		t.Fatal("After error set, no more bytes should be written")
	}
	if w.err != err1 {
		t.Fatalf("Incorrect error caught: %v", w.err)
	}

	w.String("hello")
	if len(b.Bytes()) != 1 {
		t.Fatal("After error set, no more bytes should be written")
	}
	if w.err != err1 {
		t.Fatalf("Incorrect error caught: %v", w.err)
	}

}
