package message

import (
	"fmt"
	"io"
)

type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

func NewHandshake(InfoHash [20]byte, PeerID [20]byte) *Handshake {
	return &Handshake{
		InfoHash: InfoHash,
		PeerID:   PeerID,
		Pstr:     "BitTorrent protocol",
	}
}

func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49) // 68

	buf[0] = byte(len(h.Pstr))
	curr := 1

	curr += copy(buf[curr:], h.Pstr)
	curr += 8
	curr += copy(buf[curr:], h.InfoHash[:])
	curr += copy(buf[curr:], h.PeerID[:])

	return buf
}

func ReadHandshake(r io.Reader) (*Handshake, error) {
	lengthBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}

	pstrLen := int(lengthBuf[0])
	if pstrLen == 0 {
		return nil, fmt.Errorf("pstr length is zero")
	}

	handshakeBuf := make([]byte, pstrLen+48)
	_, err = io.ReadFull(r, handshakeBuf)
	if err != nil {
		return nil, err
	}

	var InfoHash, PeerID [20]byte

	pstr := string(handshakeBuf[:pstrLen])

	copy(InfoHash[:], handshakeBuf[pstrLen+8:pstrLen+28])
	copy(PeerID[:], handshakeBuf[pstrLen+28:pstrLen+48])

	return &Handshake{
		Pstr:     pstr,
		InfoHash: InfoHash,
		PeerID:   PeerID,
	}, nil
}
