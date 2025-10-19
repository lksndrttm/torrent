package bitfield


type Bitfield []byte

func (bf Bitfield) HavePiece(pieceIdx int) bool {
	byteIdx := pieceIdx / 8
	offset := pieceIdx % 8

	if byteIdx < 0 || byteIdx >= len(bf) {
		return false
	}

	return (bf[byteIdx] >> (7-offset))&1 != 0
}

func (bf Bitfield) Len() int {
	return len(bf) * 8
}
