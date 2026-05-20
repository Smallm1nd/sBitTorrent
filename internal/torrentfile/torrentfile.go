package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"log"
	"os"

	"github.com/Smallm1nd/sBitTorrent/internal/p2p"
	"github.com/Smallm1nd/sBitTorrent/internal/peers"
	"github.com/jackpal/bencode-go"
)

const (
	portTorrent = 6881
)

type bencodeInfo struct {
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Name        string `bencode:"name"`
	Length      int    `bencode:"length"`
}

type bencodeTorrent struct {
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list"`
	Info         bencodeInfo `bencode:"info"`
}

func (i *bencodeInfo) Hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	return sha1.Sum(buf.Bytes()), nil
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	const pieceLen = 20
	buf := []byte(i.Pieces)
	if len(buf)%pieceLen != 0 {
		return nil, fmt.Errorf("malformed pieces length")
	}
	numHashes := len(buf) / pieceLen
	hashes := make([][20]byte, numHashes)
	for b := 0; b < numHashes; b++ {
		copy(hashes[b][:], buf[b*pieceLen:(b+1)*pieceLen])
	}
	return hashes, nil
}

func (t *bencodeTorrent) allTrackers() []string {
	seen := make(map[string]bool)
	var result []string
	for _, list := range t.AnnounceList {
		for _, tracker := range list {
			if tracker != "" && !seen[tracker] {
				seen[tracker] = true
				result = append(result, tracker)
			}
		}
	}
	if t.Announce != "" && !seen[t.Announce] {
		result = append(result, t.Announce)
	}
	return result
}

func (t *bencodeTorrent) ToTorrent() (*p2p.Torrent, error) {
	var PeerID [20]byte
	_, err := rand.Read(PeerID[:])
	if err != nil {
		return nil, err
	}
	InfoHash, err := t.Info.Hash()
	if err != nil {
		return nil, err
	}
	PiecesHashes, err := t.Info.splitPieceHashes()
	if err != nil {
		return nil, err
	}

	trackers := t.allTrackers()
	log.Printf("Found %d tracker(s)\n", len(trackers))

	var Peers []peers.Peer
	for _, tracker := range trackers {
		log.Printf("Trying tracker: %s\n", tracker)
		rawPeers, err := getRawPeersFromTracker(t, tracker, PeerID, portTorrent)
		if err != nil {
			log.Printf("Tracker %s failed: %v\n", tracker, err)
			continue
		}
		Peers, err = peers.Unmarshal(rawPeers)
		if err != nil {
			log.Printf("Tracker %s bad response: %v\n", tracker, err)
			continue
		}
		if len(Peers) > 0 {
			log.Printf("Tracker %s returned %d peers\n", tracker, len(Peers))
			break
		}
		log.Printf("Tracker %s returned 0 peers, trying next...\n", tracker)
	}

	result := p2p.Torrent{
		PeerId:       PeerID,
		Peers:        Peers,
		InfoHash:     InfoHash,
		PiecesHashes: PiecesHashes,
		Length:       t.Info.Length,
		PiecesLength: t.Info.PieceLength,
		Name:         t.Info.Name,
	}
	return &result, nil
}

func Open(path string) (*bencodeTorrent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	bto := new(bencodeTorrent)
	err = bencode.Unmarshal(file, bto)
	if err != nil {
		return nil, err
	}
	return bto, nil
}
