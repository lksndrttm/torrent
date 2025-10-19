package tracker

import (
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/lksndrttm/bencode"
	"github.com/lksndrttm/torrent/metadata"
	"github.com/lksndrttm/torrent/peer"
)

type trackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func (tr *trackerResponse) parsePeers() ([]peer.PeerAddr, error) {
	peersBin := []byte(tr.Peers)
	peerSize := 6
	if len(peersBin)%peerSize != 0 {
		return []peer.PeerAddr{}, errors.New("malformed peers")
	}

	numPeers := len(peersBin) / peerSize
	peers := make([]peer.PeerAddr, numPeers)

	for i := range numPeers {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16([]byte(peersBin[offset+4 : offset+6]))
	}
	return peers, nil
}

type TorrentTracker interface {
	RequestPeers(tmeta *metadata.TorrentMetadata, peerID [20]byte) ([]peer.PeerAddr, error)
}

type Tracker struct {
	URL string
}

func New(URL string) *Tracker {
	return &Tracker{URL: URL}
}

func (t *Tracker) RequestPeers(tmeta *metadata.TorrentMetadata, peerID [20]byte) ([]peer.PeerAddr, error) {
	requestURL, err := buildTrackerURL(tmeta, peerID, 6881)
	if err != nil {
		return []peer.PeerAddr{}, err
	}
	resp, err := http.Get(requestURL)
	if err != nil {
		return []peer.PeerAddr{}, err
	}
	defer resp.Body.Close()

	trackerResp := trackerResponse{}
	err = bencode.Unmarshal(&trackerResp, resp.Body)
	if err != nil {
		return []peer.PeerAddr{}, err
	}

	peers, err := trackerResp.parsePeers()

	return peers, err
}

func buildTrackerURL(t *metadata.TorrentMetadata, peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}
