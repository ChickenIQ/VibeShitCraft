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

func TestAddItemToInventory(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}

	// Add first item - should go to hotbar slot 36
	slot, ok := addItemToInventory(player, 3, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for first item")
	}
	if slot != 36 {
		t.Errorf("first item slot = %d, want 36", slot)
	}
	if player.Inventory[36].ItemID != 3 || player.Inventory[36].Count != 1 {
		t.Errorf("slot 36 = {%d, %d}, want {3, 1}", player.Inventory[36].ItemID, player.Inventory[36].Count)
	}

	// Add same item again - should stack in slot 36
	slot, ok = addItemToInventory(player, 3, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for stacking")
	}
	if slot != 36 {
		t.Errorf("stacked item slot = %d, want 36", slot)
	}
	if player.Inventory[36].Count != 2 {
		t.Errorf("slot 36 count = %d, want 2", player.Inventory[36].Count)
	}

	// Add different item - should go to next empty hotbar slot 37
	slot, ok = addItemToInventory(player, 4, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for different item")
	}
	if slot != 37 {
		t.Errorf("different item slot = %d, want 37", slot)
	}
}

func TestAddItemToInventoryFull(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}

	// Fill all inventory slots (9-44) with a non-matching item at max stack
	for i := 9; i <= 44; i++ {
		player.Inventory[i] = Slot{ItemID: 1, Count: 64}
	}

	// Try to add a different item - should fail
	_, ok := addItemToInventory(player, 3, 1)
	if ok {
		t.Error("addItemToInventory should fail when inventory is full")
	}
}

func TestAddItemStackOverflow(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}

	// Put 63 items in slot 36
	player.Inventory[36] = Slot{ItemID: 3, Count: 63}

	// Add 1 more - should stack to 64
	slot, ok := addItemToInventory(player, 3, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for stacking to 64")
	}
	if slot != 36 {
		t.Errorf("stacked item slot = %d, want 36", slot)
	}
	if player.Inventory[36].Count != 64 {
		t.Errorf("slot 36 count = %d, want 64", player.Inventory[36].Count)
	}

	// Slot is now full (64), adding more of same item should go to next slot
	slot, ok = addItemToInventory(player, 3, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for new slot after full stack")
	}
	if slot != 37 {
		t.Errorf("new slot = %d, want 37", slot)
	}
}
