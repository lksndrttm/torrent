package bitfield

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitfieldHavePiece(t *testing.T) {
	t.Parallel()
	var b1 byte = 1 << 7
	var b2 byte = 1 << 6
	bitfield := Bitfield{b1, b2}

	require.True(t, bitfield.HavePiece(0))
	require.False(t, bitfield.HavePiece(1))
	require.True(t, bitfield.HavePiece(9))
}
