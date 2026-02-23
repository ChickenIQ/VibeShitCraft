package server

import (
	"net"
	"testing"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

func TestHandleAttack(t *testing.T) {
	s := New(DefaultConfig())
	p1c1, p1c2 := net.Pipe()
	p2c1, p2c2 := net.Pipe()
	defer p1c1.Close()
	defer p1c2.Close()
	defer p2c1.Close()
	defer p2c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: p1c1, X: 0, Z: 0}
	target := &Player{EntityID: 2, Username: "Target", Health: 20.0, IsDead: false, GameMode: GameModeSurvival, Conn: p2c1, X: 1, Z: 0}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Capture packets sent to target (Server writes to target.Conn (p2c1), so we read from p2c2)
	var receivedIDs []int32
	receivedVelocity := false
	done := make(chan bool, 1)
	go func() {
		for {
			pkt, err := protocol.ReadPacket(p2c2)
			if err != nil {
				close(done)
				return
			}
			receivedIDs = append(receivedIDs, pkt.ID)
			if pkt.ID == 0x12 { // Entity Velocity
				receivedVelocity = true
				done <- true
			}
		}
	}()

	// Dummy reader for attacker to prevent blocking (Server writes to attacker.Conn (p1c1), so we read from p1c2)
	go func() {
		for {
			if _, err := protocol.ReadPacket(p1c2); err != nil {
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
	p1c1, p1c2 := net.Pipe()
	p2c1, p2c2 := net.Pipe()
	defer p1c1.Close()
	defer p1c2.Close()
	defer p2c1.Close()
	defer p2c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: p1c1}
	target := &Player{EntityID: 2, Username: "Target", Health: 2.0, IsDead: false, GameMode: GameModeSurvival, Conn: p2c1}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Dummy readers
	go func() {
		for {
			if _, err := protocol.ReadPacket(p1c2); err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			if _, err := protocol.ReadPacket(p2c2); err != nil {
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
		for {
			if _, err := protocol.ReadPacket(c2); err != nil {
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
	p1c1, p1c2 := net.Pipe()
	p2c1, p2c2 := net.Pipe()
	defer p1c1.Close()
	defer p1c2.Close()
	defer p2c1.Close()
	defer p2c2.Close()

	attacker := &Player{EntityID: 1, Username: "Attacker", Conn: p1c1}
	target := &Player{EntityID: 2, Username: "Target", Health: 20.0, IsDead: false, GameMode: GameModeCreative, Conn: p2c1}

	s.players[attacker.EntityID] = attacker
	s.players[target.EntityID] = target

	// Dummy readers
	go func() {
		for {
			if _, err := protocol.ReadPacket(p1c2); err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			if _, err := protocol.ReadPacket(p2c2); err != nil {
				return
			}
		}
	}()

	s.handleAttack(attacker, target.EntityID)

	if target.Health != 20.0 {
		t.Errorf("target health = %f, want 20.0 (creative mode)", target.Health)
	}
}
