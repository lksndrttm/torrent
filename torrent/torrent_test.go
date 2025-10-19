package torrent

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/lksndrttm/torrent/bitfield"
	m "github.com/lksndrttm/torrent/messages"
	md "github.com/lksndrttm/torrent/metadata"
	"github.com/lksndrttm/torrent/peer"
	"github.com/lksndrttm/torrent/tracker"
	"github.com/stretchr/testify/require"
)

func TestPieceDownloadingInfoAddBlock(t *testing.T) {
	t.Parallel()
	pieceLen := uint32(BlockSize * 2)
	piece := NewPieceDownloadInfo(1, pieceLen)

	pieceData := make([]byte, BlockSize)
	pieceData[0] = 1
	pmsg := m.NewPieceMessage(1, BlockSize, pieceData)
	err := piece.AddBlock(pmsg)
	require.NoError(t, err)

	require.True(t, piece.Blocks[1])
	require.Equal(t, piece.Data[BlockSize], byte(1))
}

func TestPieceDownloadInfoCompleted(t *testing.T) {
	t.Parallel()
	pieceLen := uint32(BlockSize * 2)
	piece := NewPieceDownloadInfo(1, pieceLen)

	pieceData := make([]byte, BlockSize)
	pmsg1 := m.NewPieceMessage(1, 0, pieceData)
	pmsg2 := m.NewPieceMessage(1, BlockSize, pieceData)

	err := piece.AddBlock(pmsg1)
	require.NoError(t, err)
	require.False(t, piece.Completed())

	err = piece.AddBlock(pmsg2)
	require.NoError(t, err)
	require.True(t, piece.Completed())
}

func TestTorrentDataPiece(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name()) //nolint:errcheck
	defer f.Close()           //nolint:errcheck

	testPieceLen := 2
	testData := []byte{0, 1, 2, 3, 4, 5, 6}
	r := bytes.NewReader(testData)
	tm, err := md.GenerateTorrent(r, "test", "test", testPieceLen)
	require.NoError(t, err)

	_, err = f.Write(testData)
	if err != nil {
		t.Fatal(err)
	}

	torrentData := TorrentData{
		File:            f,
		TorrentMetadata: tm,
	}

	piece, err := torrentData.Piece(1)
	require.NoError(t, err)
	expected := []byte{2, 3}

	if !slices.Equal(piece, expected) {
		t.Fatalf("Wrong piece data expected: %v actual: %v", expected, piece)
	}

	piece, err = torrentData.Piece(3)
	require.NoError(t, err)
	expected = []byte{6}

	if !slices.Equal(piece, expected) {
		t.Fatalf("Wrong piece data expected: %v actual: %v", expected, piece)
	}
}

func TestTorrentDataWritePiece(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name()) //nolint:errcheck
	defer f.Close()           //nolint:errcheck

	torrentData := TorrentData{
		File:            f,
		TorrentMetadata: &md.TorrentMetadata{PieceLength: 2, Length: 7},
	}

	err = torrentData.WritePiece(1, []byte{2, 3})
	require.NoError(t, err)
	err = torrentData.WritePiece(3, []byte{8})
	require.NoError(t, err)

	expectedP1 := []byte{2, 3}
	expectedP3 := []byte{8}

	piece1, err := torrentData.Piece(1)
	require.NoError(t, err)
	piece3, err := torrentData.Piece(3)
	require.NoError(t, err)

	if !slices.Equal(expectedP1, piece1) {
		t.Fatalf("%v != %v", expectedP1, piece1)
	}
	if !slices.Equal(expectedP3, piece3) {
		t.Fatalf("%v != %v", expectedP3, piece3)
	}
}

func TestDownload(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	tests := []struct {
		name          string
		pieceCount    int
		blocksInPiece int
		lastBlockSize int
		fails         bool
	}{
		{
			name:          "All blocks same size",
			pieceCount:    3,
			blocksInPiece: 3,
			lastBlockSize: BlockSize,
		},
		{
			name:          "Last block with different size",
			pieceCount:    3,
			blocksInPiece: 3,
			lastBlockSize: BlockSize / 3,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			tmeta, tdata, err := generateTestTorrent(tst.pieceCount, tst.blocksInPiece, BlockSize, tst.lastBlockSize)
			require.NoError(err)

			outDir, err := os.MkdirTemp("", "torrenttest")
			require.NoError(err)
			defer os.RemoveAll(outDir)

			bf := bitfield.Bitfield(make([]byte, len(tmeta.PieceHashes)))
			// heve all pieces
			for i := range bf {
				bf[i] = 255
			}
			mockHandlerFunc := newMockPeerHandler(bf, tmeta, tdata, BlockSize)
			addr, cleanup, err := startMockTCPPeer(mockHandlerFunc)
			defer cleanup()
			require.NoError(err)

			peerAddr, err := peer.ParsePeerAddr(addr)
			require.NoError(err)

			mTracker := mockTracker{
				peers: []peer.PeerAddr{peerAddr},
			}

			testTorrent := newTestTorrent(tmeta, mTracker, outDir)
			outFilePath := filepath.Join(outDir, tmeta.Name)

			testTorrent.Download()

			resData, err := os.ReadFile(outFilePath)
			require.NoError(err)

			require.True(bytes.Equal(resData, tdata))
		})
	}
}

func newTestTorrent(tmeta *md.TorrentMetadata, tr tracker.TorrentTracker, outDir string) *Torrent {
	return &Torrent{
		metadata:        tmeta,
		tracker:         tr,
		downloadingInfo: &downloadingInfo{TorrentMetadata: tmeta},
		outDir:          outDir,
		speedTracker:    NewSpeedTracker(30),
	}
}

func generateTestTorrent(pieceCount, blocksInPiece, blockSize, lastBlockSize int) (*md.TorrentMetadata, []byte, error) {
	pieceLength := BlockSize * blocksInPiece
	td := generateTestTorrentData(pieceCount, blocksInPiece, blockSize, lastBlockSize)
	r := bytes.NewReader(td)
	tm, err := md.GenerateTorrent(r, "test", "test", pieceLength)
	if err != nil {
		return nil, nil, err
	}

	return tm, td, nil
}

func generateTestTorrentData(pieceCount, blocksInPiece, blockSize, lastBlockSize int) []byte {
	dataLen := pieceCount*blockSize*blocksInPiece - (blockSize - lastBlockSize)
	resData := make([]byte, dataLen)

	// making blocks different
	for p := range pieceCount {
		for b := range blocksInPiece {
			offset := (blockSize * p * blocksInPiece) + (b * blockSize)
			bSize := blockSize
			if p == pieceCount-1 && b == blocksInPiece-1 {
				bSize = lastBlockSize
			}
			for i := range bSize {
				resData[offset+i] = byte((b + p + i) % 255)
			}
		}
	}

	return resData
}

type mockTracker struct {
	peers []peer.PeerAddr
}

func (mt mockTracker) RequestPeers(tmeta *md.TorrentMetadata, peerID [20]byte) ([]peer.PeerAddr, error) {
	return mt.peers, nil
}

func startMockTCPPeer(handlerFunc func(net.Conn)) (addr string, cleanup func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return addr, func() {}, err
	}

	go func() {
		con, err := ln.Accept()
		if err != nil {
			return
		}
		defer con.Close() //nolint:errcheck
		handlerFunc(con)
	}()

	return ln.Addr().String(), func() { ln.Close() }, nil //nolint:errcheck
}

func newMockPeerHandler(mockBitfield bitfield.Bitfield, tmeta *md.TorrentMetadata, tdata []byte, blockSize int) func(net.Conn) {
	return func(con net.Conn) {
		defer con.Close()                                //nolint:errcheck
		con.SetDeadline(time.Now().Add(time.Second * 5)) //nolint:errcheck
		h, err := m.ReadHandshake(con)
		if err != nil {
			return
		}
		_, err = con.Write(h.Serialize())
		if err != nil {
			return
		}

		bmsg := m.NewBitfieldMessage(mockBitfield)
		err = peer.SendMessage(con, bmsg.ToMessage())
		if err != nil {
			return
		}

		err = peer.SendMessage(con, m.UnchokeMessage())
		if err != nil {
			return
		}

		sendQueue := make(chan *m.PieceMessage, 20)
		blocksInPiece := tmeta.PieceLength / blockSize

		go func() {
			defer close(sendQueue)
			// Receive requests
			for range blocksInPiece * len(tmeta.PieceHashes) {
				msg, err := peer.ReceiveMessage(con)
				if err != nil {
					return
				}
				rmsg, err := m.ToRequestMessage(msg)
				if err != nil {
					return
				}
				if !mockBitfield.HavePiece(int(rmsg.PieceID)) {
					continue
				}
				pieceOffset := rmsg.PieceID * uint32(tmeta.PieceLength)
				blockPos := pieceOffset + rmsg.BlockOffset
				blockSize := BlockSize
				if int(blockPos)+BlockSize >= len(tdata) {
					blockSize = len(tdata) - int(blockPos)
				}
				pmsg := m.NewPieceMessage(rmsg.PieceID, rmsg.BlockOffset, tdata[blockPos:blockPos+uint32(blockSize)])
				sendQueue <- pmsg
			}
		}()

		for range blocksInPiece * len(tmeta.PieceHashes) {
			pmsg := <-sendQueue
			if pmsg == nil {
				return
			}
			err := peer.SendMessage(con, pmsg.ToMessage())
			if err != nil {
				return
			}
		}
	}
}
