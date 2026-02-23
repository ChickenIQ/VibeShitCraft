package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// ReadVarInt reads a variable-length integer from the reader.
// Minecraft protocol VarInts are at most 5 bytes.
func ReadVarInt(r io.Reader) (int32, int, error) {
	var result int32
	var numRead int
	buf := make([]byte, 1)
	for {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return 0, numRead, err
		}
		b := buf[0]
		result |= int32(b&0x7F) << (7 * numRead)
		numRead++
		if numRead > 5 {
			return 0, numRead, fmt.Errorf("VarInt is too big")
		}
		if (b & 0x80) == 0 {
			break
		}
	}
	return result, numRead, nil
}

// WriteVarInt writes a variable-length integer to the writer.
func WriteVarInt(w io.Writer, value int32) (int, error) {
	var buf [5]byte
	n := PutVarInt(buf[:], value)
	return w.Write(buf[:n])
}

// PutVarInt encodes a VarInt into the buffer and returns the number of bytes written.
func PutVarInt(buf []byte, value int32) int {
	uval := uint32(value)
	n := 0
	for {
		if (uval & ^uint32(0x7F)) == 0 {
			buf[n] = byte(uval)
			n++
			return n
		}
		buf[n] = byte(uval&0x7F) | 0x80
		n++
		uval >>= 7
	}
}

// VarIntSize returns the number of bytes needed to encode a VarInt.
func VarIntSize(value int32) int {
	uval := uint32(value)
	size := 0
	for {
		size++
		if (uval & ^uint32(0x7F)) == 0 {
			return size
		}
		uval >>= 7
	}
}

// ReadVarLong reads a variable-length long from the reader.
func ReadVarLong(r io.Reader) (int64, int, error) {
	var result int64
	var numRead int
	buf := make([]byte, 1)
	for {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return 0, numRead, err
		}
		b := buf[0]
		result |= int64(b&0x7F) << (7 * numRead)
		numRead++
		if numRead > 10 {
			return 0, numRead, fmt.Errorf("VarLong is too big")
		}
		if (b & 0x80) == 0 {
			break
		}
	}
	return result, numRead, nil
}

// WriteVarLong writes a variable-length long to the writer.
func WriteVarLong(w io.Writer, value int64) (int, error) {
	uval := uint64(value)
	var buf [10]byte
	n := 0
	for {
		if (uval & ^uint64(0x7F)) == 0 {
			buf[n] = byte(uval)
			n++
			break
		}
		buf[n] = byte(uval&0x7F) | 0x80
		n++
		uval >>= 7
	}
	return w.Write(buf[:n])
}

// ReadString reads a length-prefixed UTF-8 string.
func ReadString(r io.Reader) (string, error) {
	length, _, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 || length > 32767*4 {
		return "", fmt.Errorf("string length out of range: %d", length)
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// WriteString writes a length-prefixed UTF-8 string.
func WriteString(w io.Writer, s string) error {
	b := []byte(s)
	_, err := WriteVarInt(w, int32(len(b)))
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ReadUint16 reads a big-endian unsigned 16-bit integer.
func ReadUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

// WriteUint16 writes a big-endian unsigned 16-bit integer.
func WriteUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// WriteInt16 writes a big-endian signed 16-bit integer.
func WriteInt16(w io.Writer, v int16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadInt32 reads a big-endian signed 32-bit integer.
func ReadInt32(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf[:])), nil
}

// WriteInt32 writes a big-endian signed 32-bit integer.
func WriteInt32(w io.Writer, v int32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadInt64 reads a big-endian signed 64-bit integer.
func ReadInt64(r io.Reader) (int64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}

// WriteInt64 writes a big-endian signed 64-bit integer.
func WriteInt64(w io.Writer, v int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadFloat32 reads a big-endian 32-bit float.
func ReadFloat32(r io.Reader) (float32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.BigEndian.Uint32(buf[:])), nil
}

// WriteFloat32 writes a big-endian 32-bit float.
func WriteFloat32(w io.Writer, v float32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], math.Float32bits(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadFloat64 reads a big-endian 64-bit float.
func ReadFloat64(r io.Reader) (float64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.BigEndian.Uint64(buf[:])), nil
}

// WriteFloat64 writes a big-endian 64-bit float.
func WriteFloat64(w io.Writer, v float64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadBool reads a boolean.
func ReadBool(r io.Reader) (bool, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return false, err
	}
	return buf[0] != 0, nil
}

// WriteBool writes a boolean.
func WriteBool(w io.Writer, v bool) error {
	var buf [1]byte
	if v {
		buf[0] = 1
	}
	_, err := w.Write(buf[:])
	return err
}

// ReadByte reads a single byte.
func ReadByte(r io.Reader) (byte, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	return buf[0], err
}

// WriteByte writes a single byte.
func WriteByte(w io.Writer, v byte) error {
	_, err := w.Write([]byte{v})
	return err
}

// ReadUUID reads a 128-bit UUID as two int64 values.
func ReadUUID(r io.Reader) ([16]byte, error) {
	var uuid [16]byte
	_, err := io.ReadFull(r, uuid[:])
	return uuid, err
}

// WriteUUID writes a 128-bit UUID.
func WriteUUID(w io.Writer, uuid [16]byte) error {
	_, err := w.Write(uuid[:])
	return err
}

// ReadPosition reads a Minecraft position (packed X/Y/Z into int64).
func ReadPosition(r io.Reader) (x, y, z int32, err error) {
	val, err := ReadInt64(r)
	if err != nil {
		return 0, 0, 0, err
	}
	x = int32(val >> 38)
	y = int32((val >> 26) & 0xFFF)
	z = int32(val << 38 >> 38)
	return x, y, z, nil
}

// WritePosition writes a Minecraft position.
func WritePosition(w io.Writer, x, y, z int32) error {
	val := (int64(x&0x3FFFFFF) << 38) | (int64(y&0xFFF) << 26) | int64(z&0x3FFFFFF)
	return WriteInt64(w, val)
}

// WriteSlotData writes Minecraft slot data for an inventory slot.
// Pass itemID=-1 for an empty slot.
func WriteSlotData(w io.Writer, itemID int16, count byte, damage int16) error {
	if err := WriteInt16(w, itemID); err != nil {
		return err
	}
	if itemID == -1 {
		return nil
	}
	if err := WriteByte(w, count); err != nil {
		return err
	}
	if err := WriteInt16(w, damage); err != nil {
		return err
	}
	// No NBT data
	_, err := w.Write([]byte{0x00})
	return err
}
