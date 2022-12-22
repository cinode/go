package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func temporaryMemory(t *testing.T) *memory {
	return newStorageMemory()
}

func TestMemoryStorageKind(t *testing.T) {
	m := temporaryMemory(t)
	require.Equal(t, "Memory", m.kind())
}
