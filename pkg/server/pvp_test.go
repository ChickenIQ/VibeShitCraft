package server

import (
	"net"
	"testing"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

func TestHandleAttack(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: c1, X: 0, Z: 0}
	target := &Player{EntityID: 2, Username: "Target", Health: 20.0, IsDead: false, GameMode: GameModeSurvival, Conn: c2, X: 1, Z: 0}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Capture packets sent to target
	var receivedIDs []int32
	receivedVelocity := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(c2)
			if err != nil {
				t.Logf("ReadPacket error: %v", err)
				close(done)
				return
			}
			t.Logf("RECV ID: %d (0x%2X)", pkt.ID, pkt.ID)
			receivedIDs = append(receivedIDs, pkt.ID)
			if pkt.ID == 0x12 { // Entity Velocity
				receivedVelocity = true
				done <- true
			}
		}
	}()

	// Dummy reader for attacker to prevent blocking
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c1.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Give some time for the reader to start
	time.Sleep(10 * time.Millisecond)

	s.handleAttack(attacker, target.EntityID)

	select {
	case <-done:
		// Go routine finished (pipe closed or velocity received)
	case <-time.After(500 * time.Millisecond):
		t.Logf("Timed out. Received IDs: %v", receivedIDs)
		t.Error("timed out waiting for velocity packet")
	}

	if target.Health != 18.0 {
		t.Errorf("target health = %f, want 18.0", target.Health)
	}
	if !receivedVelocity {
		t.Logf("Final Received IDs: %v", receivedIDs)
		t.Error("did not receive velocity packet")
	}
}

func TestHandleDeath(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: c1}
	target := &Player{EntityID: 2, Username: "Target", Health: 2.0, IsDead: false, GameMode: GameModeSurvival, Conn: c2}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Dummy readers
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c1.Read(buf)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	s.handleAttack(attacker, target.EntityID)

	if target.Health != 0 {
		t.Errorf("target health = %f, want 0", target.Health)
	}
	if !target.IsDead {
		t.Error("target should be dead")
	}
}

func TestHandleRespawn(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	player := &Player{
		EntityID: 1,
		Username: "Player",
		Health:   0,
		IsDead:   true,
		GameMode: GameModeSurvival,
		Conn:     c1,
	}
	s.players[player.EntityID] = player

	// Dummy reader
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	s.handleRespawn(player)

	if player.Health != 20.0 {
		t.Errorf("player health = %f, want 20.0", player.Health)
	}
	if player.IsDead {
		t.Error("player should not be dead")
	}
}

func TestCreativeNoDamage(t *testing.T) {
	s := New(DefaultConfig())
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: c1}
	target := &Player{EntityID: 2, Username: "Target", Health: 20.0, IsDead: false, GameMode: GameModeCreative, Conn: c2}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Dummy readers
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c1.Read(buf)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 2048)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	s.handleAttack(attacker, target.EntityID)

	if target.Health != 20.0 {
		t.Errorf("target health = %f, want 20.0 (creative mode)", target.Health)
	}
}
