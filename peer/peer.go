package peer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lksndrttm/torrent/bitfield"
	m "github.com/lksndrttm/torrent/messages"
	md "github.com/lksndrttm/torrent/metadata"
)

type PeerAddr struct {
	IP   net.IP
	Port uint16
}

func (p *PeerAddr) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

func ParsePeerAddr(addr string) (PeerAddr, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return PeerAddr{}, err
	}

	return PeerAddr{IP: tcpAddr.IP, Port: uint16(tcpAddr.Port)}, nil
}

type Peer struct {
	Con      net.Conn
	Choking  bool
	Bitfield bitfield.Bitfield
	Addr     PeerAddr
}

func (p *Peer) SendMessage(m *m.Message) error {
	_, err := p.Con.Write(m.Serialize())
	return err
}

func (p *Peer) ReceiveMessage() (*m.Message, error) {
	msg, err := ReceiveMessage(p.Con)
	return msg, err
}

func (p *Peer) Close() {
	p.Con.Close() //nolint:errcheck
}

func HandshakePeer(peer net.Conn, tmeta *md.TorrentMetadata, peerID [20]byte) error {
	hshake := m.NewHandshake(tmeta.InfoHash, peerID)

	_, err := peer.Write(hshake.Serialize())
	if err != nil {
		return err
	}

	respHshake, err := m.ReadHandshake(peer)
	if err != nil {
		return err
	}

	if hshake.InfoHash != respHshake.InfoHash {
		return errors.New("info hashes different")
	}

	return nil
}

func ReceiveMessage(peer net.Conn) (*m.Message, error) {
	msgLenBuf := make([]byte, 4)
	_, err := io.ReadFull(peer, msgLenBuf)
	if err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(msgLenBuf)
	const maxMsgLen = 20000

	if msgLen > maxMsgLen {
		return nil, fmt.Errorf("message too long")
	}

	msgBuf := make([]byte, msgLen)
	_, err = io.ReadFull(peer, msgBuf)
	if err != nil {
		return nil, err
	}

	msg, err := m.ParseMessage(msgBuf)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func SendMessage(peer net.Conn, m *m.Message) error {
	_, err := peer.Write(m.Serialize())
	return err
}

func ReceiveBitfield(peer net.Conn) (bitfield.Bitfield, error) {
	msg, err := ReceiveMessage(peer)
	if err != nil {
		return bitfield.Bitfield{}, err
	}

	bMsg, err := m.ToBitfieldMessage(msg)
	if err != nil {
		return nil, err
	}

	return bMsg.Bitfield(), nil
}

func Connect(addr PeerAddr, tmeta *md.TorrentMetadata, timeout time.Duration, peerID [20]byte) (p *Peer, err error) {
	deadline := time.Now().Add(timeout)
	peer, err := net.DialTimeout("tcp", addr.String(), time.Second*timeout)
	if err != nil {
		return p, err
	}
	err = peer.SetDeadline(deadline)
	if err != nil {
		return p, err
	}
	defer func() {
		if err == nil {
			err = peer.SetDeadline(time.Time{})
		}
	}()

	err = HandshakePeer(peer, tmeta, peerID)
	if err != nil {
		return p, err
	}

	bitfield, err := ReceiveBitfield(peer)
	if err != nil {
		return p, err
	}

	p = &Peer{Con: peer, Choking: true, Bitfield: bitfield, Addr: addr}
	return p, err
}
