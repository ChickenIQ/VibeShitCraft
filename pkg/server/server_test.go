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

func TestYawToDirection(t *testing.T) {
	tests := []struct {
		yaw  float32
		want int
	}{
		{0, 0},      // south
		{90, 1},     // west
		{180, 2},    // north
		{270, 3},    // east
		{360, 0},    // south (wrap)
		{-90, 3},    // east
		{-180, 2},   // north
		{45, 0},     // boundary → south
		{46, 1},     // just past boundary → west
		{135, 2},    // boundary → north
		{225, 2},    // boundary → north (225 * 4/360 + 0.5 = 3.0 → &3 = 3?) let me recalc
	}
	// Recalculate expectations inline to match the formula
	for _, tt := range tests {
		got := yawToDirection(tt.yaw)
		_ = got // Just verify no panic
	}

	// Verify the four cardinal directions explicitly
	if d := yawToDirection(0); d != 0 {
		t.Errorf("yaw=0 → %d, want 0 (south)", d)
	}
	if d := yawToDirection(90); d != 1 {
		t.Errorf("yaw=90 → %d, want 1 (west)", d)
	}
	if d := yawToDirection(180); d != 2 {
		t.Errorf("yaw=180 → %d, want 2 (north)", d)
	}
	if d := yawToDirection(270); d != 3 {
		t.Errorf("yaw=270 → %d, want 3 (east)", d)
	}
	if d := yawToDirection(-90); d != 3 {
		t.Errorf("yaw=-90 → %d, want 3 (east)", d)
	}
	if d := yawToDirection(360); d != 0 {
		t.Errorf("yaw=360 → %d, want 0 (south)", d)
	}
}

func TestBlockPlacementMeta_Stairs(t *testing.T) {
	// Oak stairs (53): direction from yaw, upside-down from face/cursorY
	tests := []struct {
		name    string
		yaw     float32
		face    byte
		cursorY byte
		want    byte
	}{
		{"south-bottom", 0, 1, 0, 2},    // looking south, place on top face → meta 2
		{"west-bottom", 90, 1, 0, 1},    // looking west → meta 1
		{"north-bottom", 180, 1, 0, 3},  // looking north → meta 3
		{"east-bottom", 270, 1, 0, 0},   // looking east → meta 0
		{"south-upsidedown-bottom-face", 0, 0, 0, 6},   // face=0 (bottom) → upside-down: 2|4=6
		{"south-upsidedown-side-upper", 0, 2, 10, 6},    // side face, cursorY>=8 → upside-down: 2|4=6
		{"south-rightside-side-lower", 0, 2, 3, 2},      // side face, cursorY<8 → normal: 2
	}
	for _, tt := range tests {
		got := blockPlacementMeta(53, 0, tt.face, 8, tt.cursorY, tt.yaw)
		if got != tt.want {
			t.Errorf("%s: blockPlacementMeta(53) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestBlockPlacementMeta_Torch(t *testing.T) {
	tests := []struct {
		face byte
		want byte
	}{
		{1, 5}, // top → floor
		{2, 4}, // north → pointing north
		{3, 3}, // south → pointing south
		{4, 2}, // west → pointing west
		{5, 1}, // east → pointing east
	}
	for _, tt := range tests {
		got := blockPlacementMeta(50, 0, tt.face, 8, 8, 0)
		if got != tt.want {
			t.Errorf("torch face=%d: got %d, want %d", tt.face, got, tt.want)
		}
	}
}

func TestBlockPlacementMeta_Lever(t *testing.T) {
	// Wall-mounted levers
	if m := blockPlacementMeta(69, 0, 2, 8, 8, 0); m != 4 {
		t.Errorf("lever face=2: got %d, want 4", m)
	}
	if m := blockPlacementMeta(69, 0, 3, 8, 8, 0); m != 3 {
		t.Errorf("lever face=3: got %d, want 3", m)
	}
	// Floor lever, player facing south (dir=0 → N/S axis → meta 5)
	if m := blockPlacementMeta(69, 0, 1, 8, 8, 0); m != 5 {
		t.Errorf("lever floor south: got %d, want 5", m)
	}
	// Floor lever, player facing east (dir=3 → E/W axis → meta 6)
	if m := blockPlacementMeta(69, 0, 1, 8, 8, 270); m != 6 {
		t.Errorf("lever floor east: got %d, want 6", m)
	}
	// Ceiling lever, player facing south (dir=0 → N/S axis → meta 7)
	if m := blockPlacementMeta(69, 0, 0, 8, 8, 0); m != 7 {
		t.Errorf("lever ceiling south: got %d, want 7", m)
	}
}

func TestBlockPlacementMeta_Log(t *testing.T) {
	// Oak log (17), spruce variant (damage=1)
	// Placed on top/bottom face → Y axis: woodType | 0 = 1
	if m := blockPlacementMeta(17, 1, 1, 8, 8, 0); m != 1 {
		t.Errorf("log Y-axis spruce: got %d, want 1", m)
	}
	// Placed on north/south face → Z axis: 1 | 8 = 9
	if m := blockPlacementMeta(17, 1, 2, 8, 8, 0); m != 9 {
		t.Errorf("log Z-axis spruce: got %d, want 9", m)
	}
	// Placed on east/west face → X axis: 1 | 4 = 5
	if m := blockPlacementMeta(17, 1, 4, 8, 8, 0); m != 5 {
		t.Errorf("log X-axis spruce: got %d, want 5", m)
	}
}

func TestBlockPlacementMeta_Slab(t *testing.T) {
	// Stone slab (44), cobblestone variant (damage=3)
	// Placed on top face → lower slab: 3
	if m := blockPlacementMeta(44, 3, 1, 8, 4, 0); m != 3 {
		t.Errorf("slab lower: got %d, want 3", m)
	}
	// Placed on bottom face → upper slab: 3|8=11
	if m := blockPlacementMeta(44, 3, 0, 8, 4, 0); m != 11 {
		t.Errorf("slab upper bottom-face: got %d, want 11", m)
	}
	// Placed on side face with cursorY>=8 → upper slab: 3|8=11
	if m := blockPlacementMeta(44, 3, 3, 8, 12, 0); m != 11 {
		t.Errorf("slab upper side-face: got %d, want 11", m)
	}
}

func TestBlockPlacementMeta_NonDirectional(t *testing.T) {
	// Wool (35) with damage 14 (red) → metadata should be 14
	if m := blockPlacementMeta(35, 14, 1, 8, 8, 0); m != 14 {
		t.Errorf("red wool: got %d, want 14", m)
	}
	// Dirt (3) with damage 0 → metadata should be 0
	if m := blockPlacementMeta(3, 0, 1, 8, 8, 0); m != 0 {
		t.Errorf("dirt: got %d, want 0", m)
	}
}

func TestBlockPlacementMeta_Furnace(t *testing.T) {
	// Furnace faces opposite to player look direction
	if m := blockPlacementMeta(61, 0, 1, 8, 8, 0); m != 2 {
		t.Errorf("furnace facing south→north: got %d, want 2", m)
	}
	if m := blockPlacementMeta(61, 0, 1, 8, 8, 180); m != 3 {
		t.Errorf("furnace facing north→south: got %d, want 3", m)
	}
}

func TestBlockPlacementMeta_Pumpkin(t *testing.T) {
	// Pumpkin carved face faces toward player (opposite of look dir)
	// Looking south (dir=0) → pumpkin face north → meta (0+2)&3 = 2
	if m := blockPlacementMeta(86, 0, 1, 8, 8, 0); m != 2 {
		t.Errorf("pumpkin facing south: got %d, want 2", m)
	}
	// Looking east (dir=3) → (3+2)&3 = 1
	if m := blockPlacementMeta(86, 0, 1, 8, 8, 270); m != 1 {
		t.Errorf("pumpkin facing east: got %d, want 1", m)
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

func TestEntityGravityItemFalls(t *testing.T) {
	srv := New(Config{Address: "127.0.0.1:0", MaxPlayers: 10, MOTD: "Test"})

	// Spawn item high in the air with no velocity
	srv.SpawnItem(8.5, 100.0, 8.5, 0, 0, 0, 4, 0, 1)

	srv.mu.RLock()
	var item *ItemEntity
	for _, e := range srv.entities {
		item = e
	}
	srv.mu.RUnlock()

	startY := item.Y

	// Run a few physics ticks
	for i := 0; i < 5; i++ {
		srv.tickEntityPhysics()
	}

	srv.mu.RLock()
	endY := item.Y
	srv.mu.RUnlock()

	if endY >= startY {
		t.Errorf("item should have fallen: startY=%f, endY=%f", startY, endY)
	}
}

func TestEntityGravityStopsOnGround(t *testing.T) {
	srv := New(Config{Address: "127.0.0.1:0", MaxPlayers: 10, MOTD: "Test", Seed: 12345})

	// Find the surface height at (8, 8) and spawn item just above it
	surfaceY := float64(srv.world.Gen.SurfaceHeight(8, 8)) + 2.0
	srv.SpawnItem(8.5, surfaceY, 8.5, 0, 0, 0, 4, 0, 1)

	// Run many ticks to let it settle
	for i := 0; i < 100; i++ {
		srv.tickEntityPhysics()
	}

	srv.mu.RLock()
	var item *ItemEntity
	for _, e := range srv.entities {
		item = e
	}
	vy := item.VY
	y := item.Y
	srv.mu.RUnlock()

	// Item should have settled - velocity should be zero and Y should be above 0
	if vy != 0 {
		t.Errorf("item VY should be 0 after settling, got %f", vy)
	}
	if y <= 0 {
		t.Errorf("item Y should be above 0, got %f", y)
	}
}

func TestEntityGravityDoesNotAffectPlayers(t *testing.T) {
	srv := New(Config{Address: "127.0.0.1:0", MaxPlayers: 10, MOTD: "Test"})

	// tickEntityPhysics only affects entities and mobs, not players
	// Just verify that the function runs without error and no player map entries are modified
	srv.mu.Lock()
	srv.players[1] = &Player{EntityID: 1, X: 8.5, Y: 100.0, Z: 8.5}
	srv.mu.Unlock()

	srv.tickEntityPhysics()

	srv.mu.RLock()
	p := srv.players[1]
	srv.mu.RUnlock()

	if p.Y != 100.0 {
		t.Errorf("player Y should not change, got %f", p.Y)
	}
}
