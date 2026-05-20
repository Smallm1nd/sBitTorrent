package udp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"math/rand"
	"time"
)

const (
	lengthUdpTracker uint32 = 98
	actionAnnounce   uint32 = 1

	eventNothing       uint32 = 0
	eventEndDownload   uint32 = 1
	eventStartDownload uint32 = 2
	eventStopDownload  uint32 = 3
)

func BuildAnnouncePacket(connID uint64, infoHash [20]byte, peerID [20]byte, left uint64, port uint16) ([]byte, uint32, error) {
	var transactionID uint32 = rand.Uint32()
	var key uint32 = rand.Uint32()
	var ip uint32 = 0
	var numWant int32 = -1
	var downloaded uint64 = 0
	var uploaded uint64 = 0

	buf := bytes.NewBuffer(make([]byte, 0, lengthUdpTracker))
	err := writeProbe(buf, binary.BigEndian,
		connID,
		actionAnnounce,
		transactionID,
		infoHash,
		peerID,
		downloaded,
		left,
		uploaded,
		eventNothing,
		ip,
		key,
		numWant,
		port,
	)
	if err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), transactionID, nil
}

func AnnounceToTracker(addr string, connID uint64, infoHash [20]byte, peerID [20]byte, left uint64, port uint16) ([]byte, error) {
	datagram, transactionID, err := BuildAnnouncePacket(connID, infoHash, peerID, left, port)
	if err != nil {
		return nil, err
	}

	conn, err := ConnectToTracker(addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	for i := 0; i < maxAttemptConnect; i++ {
		err = conn.SetDeadline(time.Now().Add(time.Second * 15))
		if err != nil {
			log.Println(err)
			continue
		}

		_, err = conn.Write(datagram)
		if err != nil {
			log.Println(err)
			continue
		}

		buf := make([]byte, 2048)
		numRead, err := conn.Read(buf)
		if err != nil {
			log.Println(err)
			continue
		}
		buf = buf[:numRead]

		var respAction, respTransactionID, respInterval, respLeechers, respSeeders uint32 = 0, 0, 0, 0, 0
		err = readProbe(bytes.NewBuffer(buf[:20]), binary.BigEndian,
			&respAction,
			&respTransactionID,
			&respInterval,
			&respLeechers,
			&respSeeders)
		if err != nil {
			log.Println(err)
			continue
		}

		if transactionID != respTransactionID {
			log.Println("transactionID != respTransactionID")
			continue
		}

		if respAction != actionAnnounce {
			log.Println("invalid action")
			continue
		}

		return buf[20:], nil
	}

	return nil, errors.New("too many retries attempting to announce")
}
