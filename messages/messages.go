package messages

import (
	"encoding/binary"
	"errors"

	"github.com/lksndrttm/torrent/bitfield"
)

type msgID uint8

const (
	MsgChoke         msgID = 0
	MsgUnchoke       msgID = 1
	MsgInterested    msgID = 2
	MsgNotInterested msgID = 3
	MsgHave          msgID = 4
	MsgBitfield      msgID = 5
	MsgRequest       msgID = 6
	MsgPiece         msgID = 7
	MsgCancel        msgID = 8
)

type Message struct {
	ID      msgID
	Payload []byte
}

type RequestMessage struct {
	PieceID     uint32
	BlockOffset uint32
	BlockLength uint32
}

type PieceMessage struct {
	PieceID     uint32
	BlockOffset uint32
	Data        []byte
}

type BitfieldMessage []byte

func (msg *Message) Serialize() []byte {
	if msg == nil {
		return make([]byte, 4)
	}

	length := uint32(len(msg.Payload) + 1)
	buf := make([]byte, length+4)

	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(msg.ID)
	copy(buf[5:], msg.Payload)

	return buf
}

func (rMsg *RequestMessage) ToMessage() *Message {
	payload := make([]byte, 12)

	binary.BigEndian.PutUint32(payload[0:4], rMsg.PieceID)
	binary.BigEndian.PutUint32(payload[4:8], rMsg.BlockOffset)
	binary.BigEndian.PutUint32(payload[8:12], rMsg.BlockLength)

	msg := Message{
		ID:      MsgRequest,
		Payload: payload,
	}

	return &msg
}

func (pMsg *PieceMessage) ToMessage() *Message {
	payload := make([]byte, len(pMsg.Data)+8)

	binary.BigEndian.PutUint32(payload[0:4], pMsg.PieceID)
	binary.BigEndian.PutUint32(payload[4:8], pMsg.BlockOffset)
	copy(payload[8:], pMsg.Data)

	msg := Message{
		ID:      MsgPiece,
		Payload: payload,
	}

	return &msg
}

func (bMsg BitfieldMessage) ToMessage() *Message {
	msg := Message{
		ID:      MsgBitfield,
		Payload: make([]byte, len(bMsg)),
	}
	copy(msg.Payload, []byte(bMsg))

	return &msg
}

func (bMsg BitfieldMessage) Bitfield() bitfield.Bitfield {
	return bitfield.Bitfield(bMsg)
}

func ParseMessage(buf []byte) (*Message, error) {
	if len(buf) == 0 {
		// keep-alive
		return nil, nil
	}

	if len(buf) < 1 {
		return nil, errors.New("not a message")
	}

	msg := Message{
		ID: msgID(buf[0]),
	}

	if msg.ID > 8 {
		return nil, errors.New("not a message")
	}

	if len(buf) > 1 {
		msg.Payload = buf[1:]
	}
	return &msg, nil
}

func ToRequestMessage(msg *Message) (*RequestMessage, error) {
	if msg == nil || msg.ID != MsgRequest || len(msg.Payload) != 12 {
		return nil, errors.New("cant convert to RequestMessage")
	}

	rMsg := RequestMessage{
		PieceID:     binary.BigEndian.Uint32(msg.Payload[0:4]),
		BlockOffset: binary.BigEndian.Uint32(msg.Payload[4:8]),
		BlockLength: binary.BigEndian.Uint32(msg.Payload[8:12]),
	}

	return &rMsg, nil
}

func ToPieceMessage(msg *Message) (*PieceMessage, error) {
	if msg == nil || msg.ID != MsgPiece || len(msg.Payload) < 9 {
		return nil, errors.New("cant convert to PieceMessage")
	}

	pMsg := PieceMessage{
		PieceID:     binary.BigEndian.Uint32(msg.Payload[0:4]),
		BlockOffset: binary.BigEndian.Uint32(msg.Payload[4:8]),
		Data:        msg.Payload[8:],
	}

	return &pMsg, nil
}

func ToBitfieldMessage(msg *Message) (BitfieldMessage, error) {
	if msg == nil || msg.ID != MsgBitfield {
		return nil, errors.New("cant convert to BitfieldMessage")
	}

	return NewBitfieldMessage(msg.Payload), nil
}

func NewPieceMessage(pieceID uint32, blockOffset uint32, data []byte) *PieceMessage {
	pMsg := PieceMessage{
		PieceID:     pieceID,
		BlockOffset: blockOffset,
		Data:        data,
	}

	return &pMsg
}

func NewRequestMessage(pieceID, blockOffset, blockLength uint32) *RequestMessage {
	rMsg := RequestMessage{
		PieceID:     pieceID,
		BlockOffset: blockOffset,
		BlockLength: blockLength,
	}

	return &rMsg
}

func NewBitfieldMessage(bitfield []byte) BitfieldMessage {
	return bitfield
}

func ChokeMessage() *Message {
	msg := Message{
		ID: MsgChoke,
	}
	return &msg
}

func UnchokeMessage() *Message {
	msg := Message{
		ID: MsgUnchoke,
	}
	return &msg
}

func InterestedMessage() *Message {
	msg := Message{
		ID: MsgInterested,
	}
	return &msg
}

func NotInterestedMessage() *Message {
	msg := Message{
		ID: MsgNotInterested,
	}
	return &msg
}
