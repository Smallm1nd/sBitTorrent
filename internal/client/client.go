package client

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/Smallm1nd/sBitTorrent/internal/message"
	"github.com/Smallm1nd/sBitTorrent/internal/peers"
)

type Client struct {
	Conn net.Conn
}

func New(peer peers.Peer, peerID [20]byte, InfoHash [20]byte) (*Client, error) {
	addr := net.JoinHostPort(peer.IP.String(), strconv.Itoa(int(peer.Port)))

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}

	req := message.NewHandshake(InfoHash, peerID)

	_, err = conn.Write(req.Serialize())
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	res, err := message.ReadHandshake(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !bytes.Equal(res.InfoHash[:], InfoHash[:]) {
		_ = conn.Close()
		return nil, errors.New("invalid infohash")
	}

	return &Client{Conn: conn}, nil
}
