package metadata

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lksndrttm/bencode"
)

func GenerateTorrent(source io.Reader, announce string, name string, pieceLength int) (*TorrentMetadata, error) {
	bi := bencodeInfo{
		PieceLength: pieceLength,
		Name:        name,
	}
	bt := bencodeTorrent{
		Announce: announce,
		Info:     bi,
	}

	pieceHashes := []byte{}
	length := 0
	for {
		piece := make([]byte, pieceLength)
		n, err := io.ReadFull(source, piece)

		if n > 0 {
			pieceHash := sha1.Sum(piece[:n])
			length += n
			pieceHashes = append(pieceHashes, pieceHash[:]...)
		}

		if err != nil {
			break
		}
	}

	bt.Info.Length = length

	bt.Info.Pieces = string(pieceHashes)

	return bt.toTorrentFile()
}

func ParseTorrentFile(pathToTorrentFile string) (*TorrentMetadata, error) {
	file, err := os.Open(pathToTorrentFile)
	if err != nil {
		return nil, fmt.Errorf("torrent file (%s) parse error: %w", pathToTorrentFile, err)
	}

	bt := bencodeTorrent{}
	err = bencode.Unmarshal(&bt, file)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling torrent file (%s) data error: %w", pathToTorrentFile, err)
	}

	tmeta, err := bt.toTorrentFile()
	if err != nil {
		return nil, fmt.Errorf("bencodeTorrent to TorrentMetadata conversion error: %w", err)
	}

	return tmeta, nil
}

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type TorrentMetadata struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

func (tm *TorrentMetadata) toBencodeTorrent() (*bencodeTorrent, error) {
	bInfo := bencodeInfo{
		PieceLength: tm.PieceLength,
		Length:      tm.Length,
		Name:        tm.Name,
	}
	bTorrent := bencodeTorrent{
		Announce: tm.Announce,
		Info:     bInfo,
	}

	piecesByte := []byte{}

	for _, v := range tm.PieceHashes {
		piecesByte = append(piecesByte, v[:]...)
	}
	pieces := string(piecesByte)

	bTorrent.Info.Pieces = pieces

	return &bTorrent, nil
}

func (bt *bencodeTorrent) toTorrentFile() (*TorrentMetadata, error) {
	t := TorrentMetadata{
		Announce:    bt.Announce,
		PieceLength: bt.Info.PieceLength,
		Length:      bt.Info.Length,
		Name:        bt.Info.Name,
	}
	info, err := bencode.Marshal(&bt.Info)
	if err != nil {
		return &t, err
	}

	hasher := sha1.New()
	hasher.Write(info)

	t.InfoHash = [20]byte(hasher.Sum(nil))

	reader := strings.NewReader(bt.Info.Pieces)
	var pieceHash [20]byte
	for {
		_, err := io.ReadFull(reader, pieceHash[:])
		if err != nil {
			break
		}
		t.PieceHashes = append(t.PieceHashes, pieceHash)
	}

	return &t, nil
}
