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
	slot, ok := addItemToInventory(player, 3, 0, 1)
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
	slot, ok = addItemToInventory(player, 3, 0, 1)
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
	slot, ok = addItemToInventory(player, 4, 0, 1)
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
	_, ok := addItemToInventory(player, 3, 0, 1)
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
	slot, ok := addItemToInventory(player, 3, 0, 1)
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
	slot, ok = addItemToInventory(player, 3, 0, 1)
	if !ok {
		t.Fatal("addItemToInventory failed for new slot after full stack")
	}
	if slot != 37 {
		t.Errorf("new slot = %d, want 37", slot)
	}
}

func TestGamemodeConstants(t *testing.T) {
	if GameModeSurvival != 0 {
		t.Errorf("GameModeSurvival = %d, want 0", GameModeSurvival)
	}
	if GameModeCreative != 1 {
		t.Errorf("GameModeCreative = %d, want 1", GameModeCreative)
	}
	if GameModeAdventure != 2 {
		t.Errorf("GameModeAdventure = %d, want 2", GameModeAdventure)
	}
	if GameModeSpectator != 3 {
		t.Errorf("GameModeSpectator = %d, want 3", GameModeSpectator)
	}
}

func TestParseGameMode(t *testing.T) {
	tests := []struct {
		input string
		want  byte
		ok    bool
	}{
		{"survival", GameModeSurvival, true},
		{"Survival", GameModeSurvival, true},
		{"s", GameModeSurvival, true},
		{"0", GameModeSurvival, true},
		{"creative", GameModeCreative, true},
		{"Creative", GameModeCreative, true},
		{"c", GameModeCreative, true},
		{"1", GameModeCreative, true},
		{"adventure", GameModeAdventure, true},
		{"a", GameModeAdventure, true},
		{"2", GameModeAdventure, true},
		{"spectator", GameModeSpectator, true},
		{"sp", GameModeSpectator, true},
		{"3", GameModeSpectator, true},
		{"invalid", 0, false},
		{"", 0, false},
		{"4", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseGameMode(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseGameMode(%q) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestGameModeName(t *testing.T) {
	tests := []struct {
		mode byte
		want string
	}{
		{GameModeSurvival, "Survival"},
		{GameModeCreative, "Creative"},
		{GameModeAdventure, "Adventure"},
		{GameModeSpectator, "Spectator"},
		{255, "Unknown(255)"},
	}
	for _, tt := range tests {
		got := GameModeName(tt.mode)
		if got != tt.want {
			t.Errorf("GameModeName(%d) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestFaceOffset(t *testing.T) {
	tests := []struct {
		face       byte
		dx, dy, dz int32
	}{
		{0, 0, -1, 0}, // bottom
		{1, 0, 1, 0},  // top
		{2, 0, 0, -1}, // north
		{3, 0, 0, 1},  // south
		{4, -1, 0, 0}, // west
		{5, 1, 0, 0},  // east
	}
	for _, tt := range tests {
		x, y, z := faceOffset(10, 20, 30, tt.face)
		if x != 10+tt.dx || y != 20+tt.dy || z != 30+tt.dz {
			t.Errorf("faceOffset(10,20,30,%d) = (%d,%d,%d), want (%d,%d,%d)",
				tt.face, x, y, z, 10+tt.dx, 20+tt.dy, 30+tt.dz)
		}
	}
}

func TestPlayerDefaultGameMode(t *testing.T) {
	config := Config{
		Address:         "127.0.0.1:0",
		MaxPlayers:      10,
		MOTD:            "Test",
		DefaultGameMode: GameModeCreative,
	}
	srv := New(config)
	if srv.config.DefaultGameMode != GameModeCreative {
		t.Errorf("DefaultGameMode = %d, want %d", srv.config.DefaultGameMode, GameModeCreative)
	}
}

func TestIsValidSpawnEggType(t *testing.T) {
	// Valid mob types
	validTypes := []byte{50, 51, 52, 54, 55, 56, 57, 58, 59, 60, 61, 62,
		65, 66, 67, 68,
		90, 91, 92, 93, 94, 95, 96, 98, 100, 101, 120}
	for _, typeID := range validTypes {
		if !IsValidSpawnEggType(typeID) {
			t.Errorf("IsValidSpawnEggType(%d) = false, want true", typeID)
		}
	}

	// Invalid types
	invalidTypes := []byte{0, 1, 49, 53, 63, 64, 69, 89, 99, 102, 255}
	for _, typeID := range invalidTypes {
		if IsValidSpawnEggType(typeID) {
			t.Errorf("IsValidSpawnEggType(%d) = true, want false", typeID)
		}
	}
}

func TestSpawnEggItemID(t *testing.T) {
	if SpawnEggItemID != 383 {
		t.Errorf("SpawnEggItemID = %d, want 383", SpawnEggItemID)
	}
}

func TestServerMobsMapInitialized(t *testing.T) {
	srv := New(DefaultConfig())
	if srv.mobs == nil {
		t.Fatal("mobs map is nil after New()")
	}
	if len(srv.mobs) != 0 {
		t.Errorf("mobs map should be empty, got %d entries", len(srv.mobs))
	}
}

func TestSpawnMob(t *testing.T) {
	config := Config{
		Address:    "127.0.0.1:0",
		MaxPlayers: 10,
		MOTD:       "Test",
	}
	srv := New(config)

	srv.SpawnMob(10.5, 65.0, 20.5, 0, 0, 90) // Pig

	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if len(srv.mobs) != 1 {
		t.Fatalf("expected 1 mob, got %d", len(srv.mobs))
	}

	for _, mob := range srv.mobs {
		if mob.TypeID != 90 {
			t.Errorf("mob TypeID = %d, want 90", mob.TypeID)
		}
		if mob.X != 10.5 || mob.Y != 65.0 || mob.Z != 20.5 {
			t.Errorf("mob position = (%f, %f, %f), want (10.5, 65.0, 20.5)", mob.X, mob.Y, mob.Z)
		}
	}
}

func TestSpawnItemEntityTracking(t *testing.T) {
	config := Config{
		Address:    "127.0.0.1:0",
		MaxPlayers: 10,
		MOTD:       "Test",
	}
	srv := New(config)

	srv.SpawnItem(5.5, 10.0, 15.5, 0.1, 0.2, -0.1, 4, 0, 1) // Cobblestone

	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if len(srv.entities) != 1 {
		t.Fatalf("expected 1 item entity, got %d", len(srv.entities))
	}

	for _, item := range srv.entities {
		if item.ItemID != 4 {
			t.Errorf("item ItemID = %d, want 4", item.ItemID)
		}
		if item.X != 5.5 || item.Y != 10.0 || item.Z != 15.5 {
			t.Errorf("item position = (%f, %f, %f), want (5.5, 10.0, 15.5)", item.X, item.Y, item.Z)
		}
		if item.VX != 0.1 || item.VY != 0.2 || item.VZ != -0.1 {
			t.Errorf("item velocity = (%f, %f, %f), want (0.1, 0.2, -0.1)", item.VX, item.VY, item.VZ)
		}
	}
}
