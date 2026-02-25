package protocol

import (
	"bytes"
	"fmt"
	"io"
)

// Connection states
const (
	StateHandshaking = 0
	StateStatus      = 1
	StateLogin       = 2
	StatePlay        = 3
)

// Protocol version for Minecraft 1.8.x
const ProtocolVersion = 47

// Packet represents a Minecraft protocol packet with an ID and payload.
type Packet struct {
	ID   int32
	Data []byte
}

// ReadPacket reads a full packet from the reader.
func ReadPacket(r io.Reader) (*Packet, error) {
	length, _, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if length < 1 {
		return nil, fmt.Errorf("packet length too small: %d", length)
	}
	if length > 2097151 { // max 3-byte VarInt
		return nil, fmt.Errorf("packet length too large: %d", length)
	}

	payload := make([]byte, length)
	_, err = io.ReadFull(r, payload)
	if err != nil {
		return nil, err
	}

	pr := bytes.NewReader(payload)
	packetID, idLen, err := ReadVarInt(pr)
	if err != nil {
		return nil, err
	}

	return &Packet{
		ID:   packetID,
		Data: payload[idLen:],
	}, nil
}

// WritePacket writes a full packet to the writer.
// WritePacket writes a full packet to the writer using a single buffered write.
func WritePacket(w io.Writer, p *Packet) error {
	idSize := VarIntSize(p.ID)
	totalLen := int32(idSize + len(p.Data))

	buf := bytes.NewBuffer(make([]byte, 0, VarIntSize(totalLen)+int(totalLen)))
	WriteVarInt(buf, totalLen)
	WriteVarInt(buf, p.ID)
	buf.Write(p.Data)

	_, err := w.Write(buf.Bytes())
	return err
}

// MarshalPacket creates a Packet from a packet ID and a builder function.
func MarshalPacket(id int32, builder func(w *bytes.Buffer)) *Packet {
	var buf bytes.Buffer
	builder(&buf)
	return &Packet{
		ID:   id,
		Data: buf.Bytes(),
	}
}
