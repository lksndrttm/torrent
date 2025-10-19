package tracker

import (
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/lksndrttm/torrent/metadata"

	"github.com/stretchr/testify/require"
)

func trackerTestServer(tm *metadata.TorrentMetadata) *httptest.Server {
	infoHash := tm.InfoHash
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, t *http.Request) {
		rURL := t.URL
		values := rURL.Query()
		rInfoHash := values.Get("info_hash")
		if rInfoHash == "" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("d14:failure reason22:No info_hash key founde")) //nolint:errcheck
			return
		}

		rInfoHashBytes := []byte(rInfoHash)
		if !slices.Equal(rInfoHashBytes, infoHash[:]) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("d14:failure reason14:Hash not founde")) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusOK)

		w.Write([]byte("d8:intervali900e5:peers")) //nolint:errcheck
		w.Write([]byte("12:"))                     //nolint:errcheck
		ip := []byte{0, 0, 0, 0}
		port := []byte{0, 1}
		w.Write(append(ip, port...)) //nolint:errcheck
		ip = []byte{0, 0, 0, 1}
		port = []byte{0, 2}
		w.Write(append(ip, port...)) //nolint:errcheck
		w.Write([]byte("e"))
	}))
	return server
}

func TestTrackerRequestPeers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	sReader := strings.NewReader("1111222233334")
	tMeta, err := metadata.GenerateTorrent(sReader, "test", "test", 4)
	require.NoError(err)

	server := trackerTestServer(tMeta)
	defer server.Close()

	tMeta.Announce = server.URL

	expectedIP1 := net.IP{0, 0, 0, 0}
	var expectedPort1, expectedPort2 uint16 = 1, 2
	expectedIP2 := net.IP{0, 0, 0, 1}

	tracker := New(server.URL)

	var peerID [20]byte
	peers, err := tracker.RequestPeers(tMeta, peerID)
	require.NoError(err)
	require.Equal(len(peers), 2, "expected number of peers: 2")

	if !peers[0].IP.Equal(expectedIP1) || peers[0].Port != expectedPort1 {
		t.Fatalf("%v:%d != %v:%d", peers[0].IP, peers[0].Port, expectedIP1, expectedPort1)
	}

	if !peers[1].IP.Equal(expectedIP2) || peers[1].Port != expectedPort2 {
		t.Fatalf("%v:%d != %v:%d", peers[1].IP, peers[1].Port, expectedIP2, expectedPort2)
	}
}
