package golang

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssert(t *testing.T) {
	require.NotPanics(t, func() {
		Assert(true, "must not happen")
	})
	require.Panics(t, func() {
		Assert(false, "must panic")
	})
}
