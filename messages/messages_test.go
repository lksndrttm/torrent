package messages

import (
	"slices"
	"strings"
	"testing"
)

func TestHandshakeSerialize(t *testing.T) {
	t.Parallel()
	hshake := Handshake{Pstr: "BitTorrent protocol"}
	expected := []byte{19, 66, 105, 116, 84, 111, 114, 114, 101, 110, 116, 32, 112, 114, 111, 116, 111, 99, 111, 108, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	hBytes := hshake.Serialize()

	if !slices.Equal(expected, hBytes) {
		t.Fatalf("%v != %v", expected, hBytes)
	}
}

func TestReadHandshake(t *testing.T) {
	t.Parallel()
	var infoHash [20]byte
	peerID := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	expected := NewHandshake(infoHash, peerID)

	r := strings.NewReader(string(expected.Serialize()))

	res, err := ReadHandshake(r)
	if err != nil {
		t.Fatal(err)
	}

	if res != *expected {
		t.Fatalf("%+v != %+v", res, expected)
	}
}
