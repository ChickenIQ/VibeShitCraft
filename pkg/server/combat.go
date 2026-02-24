package server

import (
	"bytes"
	"log"
	"math"

	"github.com/StoreStation/VibeShitCraft/pkg/chat"
	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

func (s *Server) handleAttack(attacker *Player, targetID int32) {
	// Spectators cannot attack
	attacker.mu.Lock()
	if attacker.GameMode == GameModeSpectator {
		attacker.mu.Unlock()
		return
	}
	attacker.mu.Unlock()

	s.mu.RLock()
	target, ok := s.players[targetID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	target.mu.Lock()
	if target.IsDead || target.GameMode == GameModeCreative || target.GameMode == GameModeSpectator {
		target.mu.Unlock()
		return
	}

	// Calculate knockback
	attackerX, attackerZ := attacker.X, attacker.Z
	targetX, targetZ := target.X, target.Z
	target.mu.Unlock()

	// Apply damage
	damage := float32(2.0) // 1 heart
	deathMessage := "was slain by " + attacker.Username
	isDead := s.applyDamage(target, damage, deathMessage)

	// Apply knockback if not dead
	if !isDead {
		dx := targetX - attackerX
		dz := targetZ - attackerZ
		dist := math.Sqrt(dx*dx + dz*dz)

		if dist > 0 {
			// Normalize and scale
			vx := (dx / dist) * 0.4
			vz := (dz / dist) * 0.4
			vy := 0.4 // Small upward pop

			s.sendEntityVelocity(target, vx, vy, vz)
		}
	}
}

func (s *Server) applyDamage(target *Player, damage float32, deathMessage string) bool {
	target.mu.Lock()
	if target.IsDead || target.GameMode == GameModeCreative || target.GameMode == GameModeSpectator {
		target.mu.Unlock()
		return false
	}

	target.Health -= damage
	if target.Health <= 0 {
		target.Health = 0
		target.IsDead = true
	}
	isDead := target.IsDead
	target.mu.Unlock()

	// Broadcast damage animation (1 = take damage)
	s.broadcastAnimation(target, 1)
	// Broadcast hurt status (2 = hurt)
	s.broadcastEntityStatus(target.EntityID, 2)

	// Update health for the target player
	s.sendHealth(target)

	if isDead {
		// Broadcast dead status (3 = dead)
		s.broadcastEntityStatus(target.EntityID, 3)
		// Broadcast death message
		s.broadcastChat(chat.Colored(target.Username+" "+deathMessage, "red"))
		log.Printf("Player %s %s", target.Username, deathMessage)
	}

	return isDead
}

func (s *Server) sendEntityVelocity(player *Player, vx, vy, vz float64) {
	log.Printf("Sending velocity to %s: %f, %f, %f", player.Username, vx, vy, vz)
	// Entity Velocity packet (0x12)
	// Velocity is in units of 1/8000 blocks per tick.
	pkt := protocol.MarshalPacket(0x12, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, player.EntityID)
		protocol.WriteInt16(w, int16(vx*8000))
		protocol.WriteInt16(w, int16(vy*8000))
		protocol.WriteInt16(w, int16(vz*8000))
	})

	player.mu.Lock()
	protocol.WritePacket(player.Conn, pkt)
	player.mu.Unlock()
}

func (s *Server) handleRespawn(player *Player) {
	player.mu.Lock()
	if !player.IsDead {
		player.mu.Unlock()
		return
	}

	// Reset health and state
	player.Health = 20.0
	player.IsDead = false

	// Reset position to spawn (8, spawnY, 8)
	spawnY := float64(s.world.Gen.SurfaceHeight(8, 8)) + 1.0
	player.X = 8
	player.Y = spawnY
	player.Z = 8
	player.mu.Unlock()

	// 0x07 Respawn packet
	respawnPkt := protocol.MarshalPacket(0x07, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, 0) // Overworld
		protocol.WriteByte(w, 0)  // Peaceful difficulty
		protocol.WriteByte(w, player.GameMode)
		protocol.WriteString(w, "default")
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, respawnPkt)
	player.mu.Unlock()

	// Send Position
	posLook := protocol.MarshalPacket(0x08, func(w *bytes.Buffer) {
		protocol.WriteFloat64(w, player.X)
		protocol.WriteFloat64(w, player.Y)
		protocol.WriteFloat64(w, player.Z)
		protocol.WriteFloat32(w, 0)
		protocol.WriteFloat32(w, 0)
		protocol.WriteByte(w, 0) // Flags
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, posLook)
	player.mu.Unlock()

	// Update health
	s.sendHealth(player)

	// Re-spawn for others
	s.broadcastDestroyEntity(player.EntityID)
	s.spawnPlayerForOthers(player)

	log.Printf("Player %s respawned", player.Username)
}

func (s *Server) sendHealth(player *Player) {
	player.mu.Lock()
	health := player.Health
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x06, func(w *bytes.Buffer) {
		protocol.WriteFloat32(w, health)
		protocol.WriteVarInt(w, 20)   // Food
		protocol.WriteFloat32(w, 5.0) // Food Saturation
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, pkt)
	player.mu.Unlock()
}
