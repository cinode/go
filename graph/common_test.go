package graph

import "testing"

func TestPanicOn(t *testing.T) {
	mustPanic(t, func() {
		panicOn(true, "Must panic")
	})
	panicOn(false, "Must not panic")
}
