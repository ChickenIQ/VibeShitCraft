package server

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

func TestSwitchGameMode(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Tester",
		Conn:     c1,
		GameMode: GameModeCreative,
	}
	s.players[player.EntityID] = player

	// Drain packets
	go func() {
		for {
			if _, err := protocol.ReadPacket(c2); err != nil {
				return
			}
		}
	}()

	s.switchGameMode(player, GameModeSpectator)

	player.mu.Lock()
	if player.GameMode != GameModeSpectator {
		t.Errorf("GameMode = %d, want %d (Spectator)", player.GameMode, GameModeSpectator)
	}
	player.mu.Unlock()

	s.switchGameMode(player, GameModeCreative)

	player.mu.Lock()
	if player.GameMode != GameModeCreative {
		t.Errorf("GameMode = %d, want %d (Creative)", player.GameMode, GameModeCreative)
	}
	player.mu.Unlock()
}

func TestF3NCreativeToSpectator(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Tester",
		Conn:     c1,
		GameMode: GameModeCreative,
	}
	s.players[player.EntityID] = player

	// Read packets sent to player
	receivedChangeState := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				return
			}
			if pkt.ID == 0x2B { // Change Game State
				r := bytes.NewReader(pkt.Data)
				reason, _ := protocol.ReadByte(r)
				value, _ := protocol.ReadFloat32(r)
				if reason == 3 && value == float32(GameModeSpectator) {
					receivedChangeState = true
					done <- true
				}
			}
		}
	}()

	// Simulate F3+N: client sends Player Abilities without Instant Break (0x08)
	// Spectator flags: 0x07 (Invulnerable|Flying|AllowFlying, no InstantBreak)
	pkt := protocol.MarshalPacket(0x13, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0x07)    // Flags without Instant Break
		protocol.WriteFloat32(w, 0.05) // Flying speed
		protocol.WriteFloat32(w, 0.1)  // Walking speed
	})

	s.handlePlayPacket(player, pkt)

	// Wait a bit for packets
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	player.mu.Lock()
	if player.GameMode != GameModeSpectator {
		t.Errorf("GameMode = %d, want %d (Spectator) after F3+N", player.GameMode, GameModeSpectator)
	}
	player.mu.Unlock()

	if !receivedChangeState {
		t.Error("did not receive Change Game State packet for spectator mode")
	}
}

func TestF3NSpectatorToCreative(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Tester",
		Conn:     c1,
		GameMode: GameModeSpectator,
	}
	s.players[player.EntityID] = player

	receivedChangeState := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				return
			}
			if pkt.ID == 0x2B {
				r := bytes.NewReader(pkt.Data)
				reason, _ := protocol.ReadByte(r)
				value, _ := protocol.ReadFloat32(r)
				if reason == 3 && value == float32(GameModeCreative) {
					receivedChangeState = true
					done <- true
				}
			}
		}
	}()

	// Simulate F3+N: client sends Player Abilities WITH Instant Break (0x08)
	// Creative flags: 0x0D (Invulnerable|AllowFlying|InstantBreak)
	pkt := protocol.MarshalPacket(0x13, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0x0D)    // Flags with Instant Break
		protocol.WriteFloat32(w, 0.05)
		protocol.WriteFloat32(w, 0.1)
	})

	s.handlePlayPacket(player, pkt)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	player.mu.Lock()
	if player.GameMode != GameModeCreative {
		t.Errorf("GameMode = %d, want %d (Creative) after F3+N", player.GameMode, GameModeCreative)
	}
	player.mu.Unlock()

	if !receivedChangeState {
		t.Error("did not receive Change Game State packet for creative mode")
	}
}

func TestF3NNoSwitchInSurvival(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Tester",
		Conn:     c1,
		GameMode: GameModeSurvival,
	}
	s.players[player.EntityID] = player

	go func() {
		for {
			if _, err := protocol.ReadPacket(c2); err != nil {
				return
			}
		}
	}()

	// Simulate sending abilities from survival - should NOT trigger gamemode switch
	pkt := protocol.MarshalPacket(0x13, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0x00)
		protocol.WriteFloat32(w, 0.05)
		protocol.WriteFloat32(w, 0.1)
	})

	s.handlePlayPacket(player, pkt)

	player.mu.Lock()
	if player.GameMode != GameModeSurvival {
		t.Errorf("GameMode = %d, want %d (Survival) - should not change", player.GameMode, GameModeSurvival)
	}
	player.mu.Unlock()
}

func TestAdventureCannotPlaceBlocks(t *testing.T) {
	s := New(Config{Address: "127.0.0.1:0", MaxPlayers: 10, MOTD: "Test", Seed: 12345})
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Tester",
		Conn:     c1,
		GameMode: GameModeAdventure,
	}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	player.Inventory[36] = Slot{ItemID: 4, Count: 64, Damage: 0} // Cobblestone
	player.ActiveSlot = 0
	s.players[player.EntityID] = player

	// Drain packets and look for slot sync (0x2F) indicating block was rejected
	receivedSlotSync := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				return
			}
			if pkt.ID == 0x2F {
				receivedSlotSync = true
				done <- true
			}
		}
	}()

	// Simulate block placement at (10, 64, 10) on top face
	var buf bytes.Buffer
	protocol.WritePosition(&buf, 10, 64, 10)
	protocol.WriteByte(&buf, 1)             // face = top
	protocol.WriteSlotData(&buf, 4, 64, 0)  // Cobblestone
	protocol.WriteByte(&buf, 8)             // cursorX
	protocol.WriteByte(&buf, 8)             // cursorY
	protocol.WriteByte(&buf, 8)             // cursorZ

	s.handlePlayPacket(player, &protocol.Packet{ID: 0x08, Data: buf.Bytes()})

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	// The block should NOT have been placed - player should receive slot sync
	if !receivedSlotSync {
		t.Error("adventure mode player should receive slot sync (block placement rejected)")
	}

	// Verify inventory was not decremented
	player.mu.Lock()
	if player.Inventory[36].Count != 64 {
		t.Errorf("adventure mode inventory count = %d, want 64 (should not decrement)", player.Inventory[36].Count)
	}
	player.mu.Unlock()
}

func TestSpectatorEntityFlagsInvisible(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Viewer",
		Conn:     c1,
		GameMode: GameModeSurvival,
	}
	s.players[player.EntityID] = player

	spectator := &Player{
		EntityID: 2,
		Username: "Spectator",
		Conn:     nil, // spectator doesn't need a conn for this test
		GameMode: GameModeSpectator,
	}
	s.players[spectator.EntityID] = spectator

	// Look for entity metadata packet with invisible flag
	receivedInvisible := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				return
			}
			if pkt.ID == 0x1C { // Entity Metadata
				r := bytes.NewReader(pkt.Data)
				eid, _, _ := protocol.ReadVarInt(r)
				if eid == spectator.EntityID {
					header, _ := protocol.ReadByte(r)
					flags, _ := protocol.ReadByte(r)
					if header == 0x00 && (flags&0x20) != 0 {
						receivedInvisible = true
						done <- true
					}
				}
			}
		}
	}()

	s.broadcastEntityFlags(spectator)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	if !receivedInvisible {
		t.Error("spectator entity metadata should have invisible flag (0x20) set")
	}
}

func TestNonSpectatorEntityFlagsVisible(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Viewer",
		Conn:     c1,
		GameMode: GameModeSurvival,
	}
	s.players[player.EntityID] = player

	creative := &Player{
		EntityID: 2,
		Username: "Creative",
		Conn:     nil,
		GameMode: GameModeCreative,
	}
	s.players[creative.EntityID] = creative

	receivedMetadata := false
	flagsValue := byte(0xFF)
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				return
			}
			if pkt.ID == 0x1C {
				r := bytes.NewReader(pkt.Data)
				eid, _, _ := protocol.ReadVarInt(r)
				if eid == creative.EntityID {
					header, _ := protocol.ReadByte(r)
					flags, _ := protocol.ReadByte(r)
					if header == 0x00 {
						receivedMetadata = true
						flagsValue = flags
						done <- true
					}
				}
			}
		}
	}()

	s.broadcastEntityFlags(creative)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	if !receivedMetadata {
		t.Error("should receive entity metadata for creative player")
	}
	if flagsValue&0x20 != 0 {
		t.Error("creative player entity metadata should NOT have invisible flag")
	}
}
