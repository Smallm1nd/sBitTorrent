package udp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"math/rand"
	"net"
	"time"
)

const (
	maxAttemptConnect = 8
)

func ConnectToTracker(addr string) (*net.UDPConn, error) {
	trackerAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, trackerAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
func GetConnectID(addr string) (uint64, error) {

	conn, err := ConnectToTracker(addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	pack, transactionID, err := BuildConnectPacket()
	if err != nil {
		return 0, err
	}

	for i := 0; i < maxAttemptConnect; i++ {
		err = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		if err != nil {
			log.Printf("Error settings read deadline: %v, attempt %v/%v", err, i+1, maxAttemptConnect)
			continue
		}

		_, err = conn.Write(pack)
		if err != nil {
			return 0, err
		}

		buf := make([]byte, 16)
		_, err = conn.Read(buf)
		if err != nil {
			log.Printf("Error reading packet: %v, attempt %v/%v", err, i+1, maxAttemptConnect)
			continue
		}

		respConnectionID, err := ReadUdpTracker(buf, transactionID)
		if err != nil {
			log.Printf("Error reading packet: %v, attempt %v/%v", err, i+1, maxAttemptConnect)
			continue
		}

		return respConnectionID, nil
	}

	return 0, errors.New("tracker is unreachable")
}

func ReadUdpTracker(buf []byte, transactionID uint32) (uint64, error) {
	reader := bytes.NewBuffer(buf)

	var respAction uint32
	var respTransactionID uint32
	var respConnectionID uint64
	err := readProbe(reader, binary.BigEndian,
		&respAction,
		&respTransactionID,
		&respConnectionID)
	if err != nil {
		return 0, err
	}

	if respAction != 0 {
		return 0, errors.New("invalid action")
	}

	if transactionID != respTransactionID {
		return 0, errors.New("invalid transactionID")
	}

	return respConnectionID, nil

}

func BuildConnectPacket() ([]byte, uint32, error) {
	const magicID uint64 = 0x41727101980
	var action uint32 = 0
	var transactionID uint32 = rand.Uint32()

	buf := bytes.NewBuffer(make([]byte, 0, 16))

	err := writeProbe(buf, binary.BigEndian,
		magicID,
		action,
		transactionID,
	)
	if err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), transactionID, nil
}

func writeProbe(buf *bytes.Buffer, endianness binary.ByteOrder, data ...any) error {
	for _, value := range data {
		err := binary.Write(buf, endianness, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func readProbe(buf *bytes.Buffer, endianness binary.ByteOrder, data ...any) error {
	for _, value := range data {
		err := binary.Read(buf, endianness, value)
		if err != nil {
			return err
		}
	}
	return nil
}
