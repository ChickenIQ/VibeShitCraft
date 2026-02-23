package protocol

import (
	"bytes"
	"testing"
)

func TestVarInt(t *testing.T) {
	tests := []struct {
		value    int32
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xFF, 0x01}},
		{25565, []byte{0xDD, 0xC7, 0x01}},
		{2097151, []byte{0xFF, 0xFF, 0x7F}},
		{2147483647, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x07}},
		{-1, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x08}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// Test write
			var buf bytes.Buffer
			_, err := WriteVarInt(&buf, tt.value)
			if err != nil {
				t.Fatalf("WriteVarInt(%d) error: %v", tt.value, err)
			}
			if !bytes.Equal(buf.Bytes(), tt.expected) {
				t.Errorf("WriteVarInt(%d) = %v, want %v", tt.value, buf.Bytes(), tt.expected)
			}

			// Test read
			r := bytes.NewReader(tt.expected)
			val, n, err := ReadVarInt(r)
			if err != nil {
				t.Fatalf("ReadVarInt error: %v", err)
			}
			if val != tt.value {
				t.Errorf("ReadVarInt = %d, want %d", val, tt.value)
			}
			if n != len(tt.expected) {
				t.Errorf("ReadVarInt bytes read = %d, want %d", n, len(tt.expected))
			}
		})
	}
}

func TestVarIntSize(t *testing.T) {
	tests := []struct {
		value int32
		size  int
	}{
		{0, 1},
		{1, 1},
		{127, 1},
		{128, 2},
		{25565, 3},
		{2097151, 3},
		{2147483647, 5},
		{-1, 5},
	}

	for _, tt := range tests {
		if got := VarIntSize(tt.value); got != tt.size {
			t.Errorf("VarIntSize(%d) = %d, want %d", tt.value, got, tt.size)
		}
	}
}

func TestString(t *testing.T) {
	tests := []string{
		"",
		"Hello",
		"Hello, World!",
		"日本語テスト",
	}

	for _, s := range tests {
		var buf bytes.Buffer
		err := WriteString(&buf, s)
		if err != nil {
			t.Fatalf("WriteString(%q) error: %v", s, err)
		}

		r := bytes.NewReader(buf.Bytes())
		got, err := ReadString(r)
		if err != nil {
			t.Fatalf("ReadString error: %v", err)
		}
		if got != s {
			t.Errorf("ReadString = %q, want %q", got, s)
		}
	}
}

func TestPacketRoundTrip(t *testing.T) {
	original := &Packet{
		ID:   0x00,
		Data: []byte("test data"),
	}

	var buf bytes.Buffer
	err := WritePacket(&buf, original)
	if err != nil {
		t.Fatalf("WritePacket error: %v", err)
	}

	r := bytes.NewReader(buf.Bytes())
	got, err := ReadPacket(r)
	if err != nil {
		t.Fatalf("ReadPacket error: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("Packet ID = %d, want %d", got.ID, original.ID)
	}
	if !bytes.Equal(got.Data, original.Data) {
		t.Errorf("Packet Data = %v, want %v", got.Data, original.Data)
	}
}

func TestInt32(t *testing.T) {
	values := []int32{0, 1, -1, 2147483647, -2147483648, 42}
	for _, v := range values {
		var buf bytes.Buffer
		err := WriteInt32(&buf, v)
		if err != nil {
			t.Fatalf("WriteInt32(%d) error: %v", v, err)
		}
		r := bytes.NewReader(buf.Bytes())
		got, err := ReadInt32(r)
		if err != nil {
			t.Fatalf("ReadInt32 error: %v", err)
		}
		if got != v {
			t.Errorf("ReadInt32 = %d, want %d", got, v)
		}
	}
}

func TestFloat64(t *testing.T) {
	values := []float64{0, 1.5, -1.5, 3.14159265}
	for _, v := range values {
		var buf bytes.Buffer
		err := WriteFloat64(&buf, v)
		if err != nil {
			t.Fatalf("WriteFloat64(%f) error: %v", v, err)
		}
		r := bytes.NewReader(buf.Bytes())
		got, err := ReadFloat64(r)
		if err != nil {
			t.Fatalf("ReadFloat64 error: %v", err)
		}
		if got != v {
			t.Errorf("ReadFloat64 = %f, want %f", got, v)
		}
	}
}

func TestBool(t *testing.T) {
	for _, v := range []bool{true, false} {
		var buf bytes.Buffer
		err := WriteBool(&buf, v)
		if err != nil {
			t.Fatalf("WriteBool(%v) error: %v", v, err)
		}
		r := bytes.NewReader(buf.Bytes())
		got, err := ReadBool(r)
		if err != nil {
			t.Fatalf("ReadBool error: %v", err)
		}
		if got != v {
			t.Errorf("ReadBool = %v, want %v", got, v)
		}
	}
}

func TestPosition(t *testing.T) {
	tests := []struct {
		x, y, z int32
	}{
		{0, 0, 0},
		{8, 64, 8},
		{-1, 0, -1},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		err := WritePosition(&buf, tt.x, tt.y, tt.z)
		if err != nil {
			t.Fatalf("WritePosition error: %v", err)
		}
		r := bytes.NewReader(buf.Bytes())
		x, y, z, err := ReadPosition(r)
		if err != nil {
			t.Fatalf("ReadPosition error: %v", err)
		}
		if x != tt.x || y != tt.y || z != tt.z {
			t.Errorf("ReadPosition = (%d, %d, %d), want (%d, %d, %d)", x, y, z, tt.x, tt.y, tt.z)
		}
	}
}

func TestMarshalPacket(t *testing.T) {
	pkt := MarshalPacket(0x01, func(w *bytes.Buffer) {
		WriteString(w, "hello")
	})

	if pkt.ID != 0x01 {
		t.Errorf("Packet ID = %d, want %d", pkt.ID, 0x01)
	}

	// Verify the data can be read back as a string
	r := bytes.NewReader(pkt.Data)
	s, err := ReadString(r)
	if err != nil {
		t.Fatalf("ReadString error: %v", err)
	}
	if s != "hello" {
		t.Errorf("ReadString = %q, want %q", s, "hello")
	}
}

func TestVarLong(t *testing.T) {
	tests := []int64{0, 1, -1, 9223372036854775807, -9223372036854775808}

	for _, v := range tests {
		var buf bytes.Buffer
		_, err := WriteVarLong(&buf, v)
		if err != nil {
			t.Fatalf("WriteVarLong(%d) error: %v", v, err)
		}
		r := bytes.NewReader(buf.Bytes())
		got, _, err := ReadVarLong(r)
		if err != nil {
			t.Fatalf("ReadVarLong error: %v", err)
		}
		if got != v {
			t.Errorf("ReadVarLong = %d, want %d", got, v)
		}
	}
}
