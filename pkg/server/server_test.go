package server

import (
	"testing"
)

func TestOfflineUUID(t *testing.T) {
	// Should be deterministic
	uuid1 := offlineUUID("TestPlayer")
	uuid2 := offlineUUID("TestPlayer")

	if uuid1 != uuid2 {
		t.Errorf("offlineUUID not deterministic: %v != %v", uuid1, uuid2)
	}

	// Different names should produce different UUIDs
	uuid3 := offlineUUID("OtherPlayer")
	if uuid1 == uuid3 {
		t.Errorf("different names produced same UUID")
	}

	// UUID version should be 3
	version := (uuid1[6] >> 4) & 0x0F
	if version != 3 {
		t.Errorf("UUID version = %d, want 3", version)
	}

	// UUID variant should be RFC 4122
	variant := (uuid1[8] >> 6) & 0x03
	if variant != 2 {
		t.Errorf("UUID variant = %d, want 2", variant)
	}
}

func TestFormatUUID(t *testing.T) {
	uuid := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	got := formatUUID(uuid)
	expected := "01020304-0506-0708-090a-0b0c0d0e0f10"
	if got != expected {
		t.Errorf("formatUUID = %q, want %q", got, expected)
	}
}

func TestNewServer(t *testing.T) {
	config := DefaultConfig()
	srv := New(config)
	if srv == nil {
		t.Fatal("New() returned nil")
	}
	if srv.config.Address != ":25565" {
		t.Errorf("Address = %q, want %q", srv.config.Address, ":25565")
	}
	if srv.config.MaxPlayers != 20 {
		t.Errorf("MaxPlayers = %d, want %d", srv.config.MaxPlayers, 20)
	}
}

func TestServerStartStop(t *testing.T) {
	config := Config{
		Address:    "127.0.0.1:0", // Use port 0 for random available port
		MaxPlayers: 10,
		MOTD:       "Test Server",
	}
	srv := New(config)
	err := srv.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	addr := srv.listener.Addr().String()
	if addr == "" {
		t.Error("Server listener address is empty")
	}
}

func TestPlayerCount(t *testing.T) {
	srv := New(DefaultConfig())
	if srv.playerCount() != 0 {
		t.Errorf("playerCount() = %d, want 0", srv.playerCount())
	}
}
