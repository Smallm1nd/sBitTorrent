package p2p

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"log"
	"sync"

	"github.com/Smallm1nd/sBitTorrent/internal/bitfield"
	"github.com/Smallm1nd/sBitTorrent/internal/client"
	"github.com/Smallm1nd/sBitTorrent/internal/message"
	"github.com/Smallm1nd/sBitTorrent/internal/peers"
)

type peerState struct {
	client   *client.Client
	bitfield bitfield.Bitfield
	choked   bool
}
type pieceWork struct {
	Index  int
	Hash   [20]byte
	Length int
}
type pieceResult struct {
	Index int
	Buf   []byte
}

type Torrent struct {
	Peers        []peers.Peer
	PeerId       [20]byte
	InfoHash     [20]byte
	PiecesHashes [][20]byte
	Length       int
	PiecesLength int
	Name         string
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin := index * t.PiecesLength

	end := begin + t.PiecesLength

	if end > t.Length {
		return t.Length - begin
	}

	return t.PiecesLength
}

func (t *Torrent) peerWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	state, err := t.initPeer(peer)
	if err != nil {
		log.Printf("Worker %s died on connect: %v\n", peer.IP, err)
		return
	}
	defer func() { _ = state.client.Conn.Close() }()
	log.Printf("connected to peer %s \n", peer.IP)

	msg, err := state.Read()
	if err != nil {
		log.Printf("error reading from peer %s\n", peer.IP)
		return
	}
	if msg != nil && msg.ID == message.MsgBitfield {
		state.bitfield = msg.Payload
	}

	err = state.Send(&message.Message{ID: message.MsgInterested})
	if err != nil {
		log.Printf("error sending message to peer %s\n", peer.IP)
		return
	}
	for piece := range workQueue {
		if !state.bitfield.HasPiece(piece.Index) {
			workQueue <- piece
			continue
		}

		buf, err := attemptDownloadPiece(state, piece)
		if err != nil {
			log.Printf("error downloading piece %d from peer %s\n", piece.Index, peer.IP)
			workQueue <- piece
			return
		}

		results <- &pieceResult{Index: piece.Index, Buf: buf}
	}

	return
}

func attemptDownloadPiece(state *peerState, pw *pieceWork) ([]byte, error) {
	buf := make([]byte, pw.Length)
	downloaded := 0
	requested := 0
	backlog := 0

	for downloaded < pw.Length {
		for !state.choked && backlog < 5 && requested < pw.Length {
			blockSize := 16384
			if pw.Length-requested < blockSize {
				blockSize = pw.Length - requested
			}
			req := make([]byte, 12)
			binary.BigEndian.PutUint32(req, uint32(pw.Index))
			binary.BigEndian.PutUint32(req[4:], uint32(requested))
			binary.BigEndian.PutUint32(req[8:], uint32(blockSize))
			err := state.Send(&message.Message{ID: message.MsgRequest, Payload: req})
			if err != nil {
				log.Printf("can't send request to peer :%s\n", err)
				return nil, err
			}
			backlog++
			requested += blockSize

		}
		msg, err := state.Read()
		if err != nil {
			log.Printf("error reading from peer :%s\n", err)
			return nil, err
		}

		if msg == nil {
			continue
		}

		switch msg.ID {
		case message.MsgUnchoke:
			state.choked = false
		case message.MsgChoke:
			state.choked = true
			backlog = 0
			requested = downloaded
		case message.MsgPiece:
			offset := binary.BigEndian.Uint32(msg.Payload[4:8])
			copy(buf[offset:], msg.Payload[8:])

			downloaded += len(msg.Payload[8:])
			backlog--
		}

	}
	if sha1.Sum(buf) != pw.Hash {
		return nil, errors.New("piece hash does not match")
	}

	return buf, nil
}

func (t *Torrent) Download() ([]byte, error) {
	log.Printf("Starting download from %d peers\n", len(t.Peers))
	if len(t.Peers) == 0 {
		return nil, errors.New("tracker returned 0 peers, nowhere to download from")
	}

	numPieces := len(t.PiecesHashes)
	workQueue := make(chan *pieceWork, numPieces)
	result := make(chan *pieceResult)

	for index, hash := range t.PiecesHashes {
		length := t.calculatePieceSize(index)

		workQueue <- &pieceWork{Index: index, Hash: hash, Length: length}
	}

	var wg sync.WaitGroup
	for _, peer := range t.Peers {
		wg.Add(1)
		go func(p peers.Peer) {
			defer wg.Done()
			t.peerWorker(p, workQueue, result)
		}(peer)
	}
	go func() {
		wg.Wait()
		close(result)
	}()

	buf := make([]byte, t.Length)
	for donePieces := 0; donePieces < numPieces; donePieces++ {
		res, ok := <-result
		if !ok {
			return nil, errors.New("all workers died before download finished")
		}

		begin := res.Index * t.PiecesLength

		copy(buf[begin:], res.Buf)

		log.Printf("Downloaded %d/%d pieces\n", donePieces+1, numPieces)
	}
	close(workQueue)

	return buf, nil
}

func (t *Torrent) initPeer(peer peers.Peer) (*peerState, error) {
	c, err := client.New(peer, t.PeerId, t.InfoHash)
	if err != nil {
		return nil, err
	}

	return &peerState{client: c, bitfield: make(bitfield.Bitfield, ((len(t.PiecesHashes))+7)/8), choked: true}, nil
	//take size with reserve
}

func (st *peerState) Read() (*message.Message, error) {
	return message.Read(st.client.Conn)
}

func (st *peerState) Send(msg *message.Message) error {
	_, err := st.client.Conn.Write(msg.Serialize())
	return err
}
