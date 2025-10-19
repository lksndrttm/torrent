package messages

import (
	"errors"
	"io"
)

type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

func NewHandshake(infoHash [20]byte, peerID [20]byte) *Handshake {
	h := Handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID:   peerID,
	}
	return &h
}

func (h *Handshake) Serialize() []byte {
	hBytes := make([]byte, 68)
	hBytes[0] = byte(len(h.Pstr))
	curr := 1
	curr += copy(hBytes[curr:], []byte(h.Pstr))
	curr += copy(hBytes[curr:], []byte{0, 0, 0, 0, 0, 0, 0, 0})
	curr += copy(hBytes[curr:], h.InfoHash[:])
	curr += copy(hBytes[curr:], h.PeerID[:])

	return hBytes
}

func ReadHandshake(r io.Reader) (h Handshake, err error) {
	hBytes := make([]byte, 68)
	_, err = io.ReadFull(r, hBytes)
	if err != nil {
		return h, err
	}
	if uint8(hBytes[0]) != 19 {
		return h, errors.New("Handshake format error")
	}
	h.Pstr = string(hBytes[1:20])
	if h.Pstr != "BitTorrent protocol" {
		return h, errors.New("Handshake format error")
	}
	copy(h.InfoHash[:], hBytes[28:48])
	copy(h.PeerID[:], hBytes[48:68])

	return h, nil
}
