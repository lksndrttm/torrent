package metadata

import (
	"crypto/sha1"
	"slices"
	"strings"
	"testing"
)

func TestGenerateTorrentPieceHashes(t *testing.T) {
	t.Parallel()
	source := strings.NewReader("1111222233334")
	tMeta, err := GenerateTorrent(source, "test", "test", 4)
	if err != nil {
		t.Error(err)
	}

	pSHA := [][20]byte{}
	pSHA = append(pSHA, sha1.Sum([]byte("1111")))
	pSHA = append(pSHA, sha1.Sum([]byte("2222")))
	pSHA = append(pSHA, sha1.Sum([]byte("3333")))
	pSHA = append(pSHA, sha1.Sum([]byte("4")))

	if len(pSHA) != len(tMeta.PieceHashes) {
		t.Errorf("Wrong number of pieces")
	}

	for i := range len(pSHA) {
		if !slices.Equal(pSHA[i][:], tMeta.PieceHashes[i][:]) {
			t.Errorf("Piece num %d %v != %v", i, pSHA[i], tMeta.PieceHashes[i])
		}
	}
}
