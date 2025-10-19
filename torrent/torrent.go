package torrent

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	m "github.com/lksndrttm/torrent/messages"
	md "github.com/lksndrttm/torrent/metadata"
	"github.com/lksndrttm/torrent/peer"
	"github.com/lksndrttm/torrent/tracker"
)

var PeerID = [20]byte([]byte("-xx2940-k8xj0xgex6xx"))

const BlockSize = 16384

type Piece struct {
	ID   uint32
	Data []byte
}

func (p *Piece) CheckIntegrity(pieceHash [20]byte) bool {
	return sha1.Sum(p.Data) == pieceHash
}

type pieceDownloadingInfo struct {
	ID          uint32
	Hash        []byte
	Blocks      []bool
	Data        []byte
	maxPieceLen uint32
	pieceLen    uint32
	blocksCount uint32
	downloaded  int
	requested   int
}

func NewPieceDownloadInfo(id uint32, maxPieceLen uint32) *pieceDownloadingInfo {
	p := pieceDownloadingInfo{
		ID:          id,
		Blocks:      make([]bool, maxPieceLen/BlockSize),
		Data:        make([]byte, maxPieceLen),
		maxPieceLen: maxPieceLen,
	}
	return &p
}

func (p *pieceDownloadingInfo) AddBlock(pmsg *m.PieceMessage) error {
	if pmsg.PieceID != p.ID {
		return fmt.Errorf("attemp to add block from another piece")
	}
	if uint32(len(p.Data)) < pmsg.BlockOffset+uint32(len(pmsg.Data)) {
		return fmt.Errorf("piece max lenght exceeded")
	}

	blockID := pmsg.BlockOffset / BlockSize
	if p.Blocks[blockID] {
		return fmt.Errorf("attemp to rewrite existing block")
	}

	copy(p.Data[pmsg.BlockOffset:], pmsg.Data)
	p.pieceLen += uint32(len(pmsg.Data))
	p.blocksCount++

	p.Blocks[blockID] = true

	return nil
}

func (p *pieceDownloadingInfo) Completed() bool {
	return p.blocksCount == p.maxPieceLen/BlockSize
}

func (p *pieceDownloadingInfo) Piece() *Piece {
	return &Piece{
		ID:   p.ID,
		Data: p.Data[:p.pieceLen],
	}
}

var (
	errNetwork     error = errors.New("network error")
	errDownloading error = errors.New("downloading error")
)

func downloadPiece(pieceID uint32, p *peer.Peer, tmeta *md.TorrentMetadata) (piece *Piece, err error) {
	have := p.Bitfield.HavePiece(int(pieceID))
	if !have {
		return nil, fmt.Errorf("peer dont have requested piece %w", errDownloading)
	}

	pdInfo := NewPieceDownloadInfo(pieceID, uint32(tmeta.PieceLength))

	requestTimeout := 10 * time.Second
	backlog := 0
	maxBacklog := 5

	blocksCount := uint32(tmeta.PieceLength) / BlockSize
	pieceOffset := uint32(tmeta.PieceLength) * pieceID
	for pdInfo.downloaded < int(blocksCount) {
		blockOffset := uint32(pdInfo.requested * BlockSize)
		offset := pieceOffset + blockOffset
		blockLength := uint32(BlockSize)
		if offset+blockLength > uint32(tmeta.Length) {
			blockLength = uint32(tmeta.Length) - offset
		}

		if err = p.Con.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
			return nil, fmt.Errorf("cant set deadline for peer io: %w", errNetwork)
		}

		rmsg := m.NewRequestMessage(pieceID, blockOffset, blockLength)
		if !p.Choking && backlog < maxBacklog && pdInfo.requested < int(blocksCount) {
			err := p.SendMessage(rmsg.ToMessage())
			if err != nil {
				return nil, fmt.Errorf("piece request sending error: %w", errNetwork)
			}
			pdInfo.requested++
			backlog++
			continue
		}

		// handle message
		msg, err := p.ReceiveMessage()
		if err != nil {
			return nil, fmt.Errorf("message receive error: %w", errNetwork)
		}

		if msg == nil {
			err := p.SendMessage(nil)
			if err != nil {
				return nil, fmt.Errorf("error while sending keep-alive message: %w", err)
			}
		}

		switch msg.ID {
		case m.MsgPiece:
			backlog--
			pdInfo.downloaded++
			pmsg, err := m.ToPieceMessage(msg)
			if err != nil {
				return nil, fmt.Errorf("cant convert received message to PieceMessage: %w", err)
			}

			err = pdInfo.AddBlock(pmsg)
			if err != nil {
				return nil, fmt.Errorf("piece %d  constructing error: %w", pdInfo.ID, err)
			}
		case m.MsgChoke:
			p.Choking = true
		case m.MsgUnchoke:
			p.Choking = false
		}

	}
	if !pdInfo.Completed() {
		return nil, fmt.Errorf("piece %d downloading attemp failed: %w", pieceID, errDownloading)
	}
	piece = pdInfo.Piece()
	if !piece.CheckIntegrity(tmeta.PieceHashes[pieceID]) {
		return nil, fmt.Errorf("piece %d failed integrity check: %w", pieceID, errDownloading)
	}

	return piece, err
}

func communicateWithPeer(peerAddr peer.PeerAddr, tm *md.TorrentMetadata, reqChan chan uint32, pieceChan chan *Piece) {
	cTimeout := time.Second * 5
	p, err := peer.Connect(peerAddr, tm, cTimeout, PeerID)
	if err != nil {
		return
	}
	defer p.Close()

	for pieceID := range reqChan {
		piece, err := downloadPiece(pieceID, p, tm)
		if err != nil {
			reqChan <- pieceID
			if errors.Is(err, errNetwork) {
				return
			}
			if errors.Is(err, errDownloading) {
				continue
			}
		}

		pieceChan <- piece
	}
}

func calcPieceBoundaries(pieceID uint32, tmeta *md.TorrentMetadata) (begin, end int) {
	begin = int(pieceID) * tmeta.PieceLength
	end = begin + tmeta.PieceLength
	end = min(end, tmeta.Length)
	return begin, end
}

type TorrentData struct {
	File            *os.File
	TorrentMetadata *md.TorrentMetadata
}

func (td *TorrentData) Piece(id int) ([]byte, error) {
	beg, end := calcPieceBoundaries(uint32(id), td.TorrentMetadata)
	pieceLen := end - beg
	piece := make([]byte, pieceLen)
	_, err := td.File.ReadAt(piece, int64(beg))
	if err != nil {
		return nil, err
	}
	return piece, nil
}

func (td *TorrentData) WritePiece(id int, piece []byte) error {
	offset := int64(id * td.TorrentMetadata.PieceLength)
	_, err := td.File.WriteAt(piece, offset)
	if err != nil {
		return err
	}
	return nil
}

type downloadingInfo struct {
	TorrentMetadata *md.TorrentMetadata
	downloaded      int
	isDone          bool
}

func (di *downloadingInfo) PieceDownloaded(piece *Piece) {
	di.downloaded += len(piece.Data)
	if di.downloaded == di.TorrentMetadata.Length {
		di.isDone = true
	}
}

func (di *downloadingInfo) Remainded() int {
	return di.TorrentMetadata.Length - di.downloaded
}

func (di *downloadingInfo) Downloaded() int {
	return di.downloaded
}

func (t *Torrent) download() {
	peers, err := t.tracker.RequestPeers(t.metadata, PeerID)
	if err != nil {
		log.Fatal(err)
		return
	}

	pieceCount := len(t.metadata.PieceHashes)

	reqChan := make(chan uint32, pieceCount)
	pieceChan := make(chan *Piece, 100)
	defer close(reqChan)

	for i := range pieceCount {
		reqChan <- uint32(i)
	}

	outFilePath := filepath.Join(t.outDir, t.metadata.Name)
	file, err := os.Create(outFilePath)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer file.Close()

	tdata := TorrentData{
		File:            file,
		TorrentMetadata: t.metadata,
	}

	for _, peer := range peers[:min(len(peers), 50)] {
		go communicateWithPeer(peer, t.metadata, reqChan, pieceChan)
	}

	for piece := range pieceChan {
		err = tdata.WritePiece(int(piece.ID), piece.Data)
		if err != nil {
			log.Fatal("Downloding error: Cant write piece")
		}
		t.downloadingInfo.PieceDownloaded(piece)
		t.speedTracker.updateDownloadingSpeed(len(piece.Data))

		if t.downloadingInfo.isDone {
			close(pieceChan)
		}
	}
}

type trackerRecord struct {
	downloaded     int
	elapsedSeconds float64
}

type speedTracker struct {
	lastDownloadSpeedUpdate time.Time
	history                 []trackerRecord
	maxHistoryLen           int
	speedRecordCount        int
	m                       sync.Mutex
}

func NewSpeedTracker(avgOf int) *speedTracker {
	return &speedTracker{
		maxHistoryLen: avgOf,
	}
}

func (st *speedTracker) updateDownloadingSpeed(bytesDownloaded int) {
	st.m.Lock()
	defer st.m.Unlock()

	now := time.Now()
	if st.lastDownloadSpeedUpdate.IsZero() {
		st.lastDownloadSpeedUpdate = now
		return
	}

	timeDiff := now.Sub(st.lastDownloadSpeedUpdate).Seconds()
	if timeDiff > 0 {
		record := trackerRecord{downloaded: bytesDownloaded, elapsedSeconds: timeDiff}
		if len(st.history) > st.maxHistoryLen {
			st.history[st.speedRecordCount%st.maxHistoryLen] = record
		} else {
			st.history = append(st.history, record)
		}
	}
	st.lastDownloadSpeedUpdate = now
	st.speedRecordCount++
}

func (st *speedTracker) DownloadingSpeed() int {
	st.m.Lock()
	defer st.m.Unlock()
	if len(st.history) == 0 {
		return 0
	}
	totalDownloaded := int64(0)
	totlaElapsed := float64(0)

	for _, s := range st.history {
		totalDownloaded += int64(s.downloaded)
		totlaElapsed += s.elapsedSeconds
	}
	return int(float64(totalDownloaded) / totlaElapsed)
}

type Torrent struct {
	metadata        *md.TorrentMetadata
	tracker         tracker.TorrentTracker
	downloadingInfo *downloadingInfo
	startTime       time.Time
	outDir          string
	speedTracker    *speedTracker
}

func New(torrentFilePath, outDir string) (*Torrent, error) {
	tmeta, err := md.ParseTorrentFile(torrentFilePath)
	if err != nil {
		return nil, err
	}

	tr := tracker.New(tmeta.Announce)

	return &Torrent{
		metadata:        tmeta,
		tracker:         tr,
		downloadingInfo: &downloadingInfo{TorrentMetadata: tmeta},
		outDir:          outDir,
		speedTracker:    NewSpeedTracker(30),
	}, nil
}

func (t *Torrent) Name() string {
	return t.metadata.Name
}

func (t *Torrent) Length() int {
	return t.metadata.Length
}

func (t *Torrent) Start() {
	t.startTime = time.Now()
	go t.download()
}

func (t *Torrent) Download() {
	t.startTime = time.Now()
	t.download()
}

func (t *Torrent) DownloadingSpeed() int {
	return t.speedTracker.DownloadingSpeed()
}

func (t *Torrent) Downloaded() int {
	return t.downloadingInfo.Downloaded()
}
