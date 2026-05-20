package torrentfile

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Smallm1nd/sBitTorrent/internal/udp"
	"github.com/jackpal/bencode-go"
)

type trackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func percentEncode(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

func buildTrackerURL(announceURL string, infoHash [20]byte, peerID [20]byte, length int, port uint16) (string, error) {
	base, err := url.Parse(announceURL)
	if err != nil {
		return "", err
	}

	// Обычные параметры — через url.Values
	params := url.Values{}
	params.Set("port", strconv.Itoa(int(port)))
	params.Set("uploaded", "0")
	params.Set("downloaded", "0")
	params.Set("left", strconv.Itoa(length))
	params.Set("compact", "1")

	query := params.Encode()
	query += "&info_hash=" + percentEncode(infoHash[:])
	query += "&peer_id=" + percentEncode(peerID[:])
	base.RawQuery = query

	return base.String(), nil
}

func getRawHttpPeers(announceURL string, infoHash [20]byte, peerID [20]byte, length int, port uint16) ([]byte, error) {
	u, err := buildTrackerURL(announceURL, infoHash, peerID, length, port)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sBitTorrent")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	defer func() { _ = resp.Body.Close() }()

	trackerResp := trackerResponse{}
	if err = bencode.Unmarshal(resp.Body, &trackerResp); err != nil {
		return nil, err
	}
	return []byte(trackerResp.Peers), nil
}

func getRawUdpPeers(announceURL string, infoHash [20]byte, peerID [20]byte, length int, port uint16) ([]byte, error) {
	u, err := url.Parse(announceURL)
	if err != nil {
		return nil, err
	}
	connectionID, err := udp.GetConnectID(u.Host)
	if err != nil {
		return nil, err
	}
	return udp.AnnounceToTracker(u.Host, connectionID, infoHash, peerID, uint64(length), port)
}

func getRawPeersFromTracker(t *bencodeTorrent, tracker string, peerID [20]byte, port uint16) ([]byte, error) {
	infoHash, err := t.Info.Hash()
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(tracker, "udp://"):
		return getRawUdpPeers(tracker, infoHash, peerID, t.Info.Length, port)
	case strings.HasPrefix(tracker, "http://"), strings.HasPrefix(tracker, "https://"):
		return getRawHttpPeers(tracker, infoHash, peerID, t.Info.Length, port)
	default:
		return nil, fmt.Errorf("unknown tracker protocol: %s", tracker)
	}
}
