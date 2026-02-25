package server

import (
	"bytes"
	"log"
	"math"
	"strings"

	"github.com/VibeShit/VibeShitCraft/pkg/chat"
	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
	"github.com/VibeShit/VibeShitCraft/pkg/world"
)

func (s *Server) handlePlayPacket(player *Player, pkt *protocol.Packet) {
	r := bytes.NewReader(pkt.Data)

	switch pkt.ID {
	case 0x00: // Keep Alive
	// Client responding to keep alive, ignore

	case 0x01: // Chat Message
		message, err := protocol.ReadString(r)
		if err != nil {
			return
		}
		if len(message) > 256 {
			message = message[:256]
		}
		// Route commands (messages starting with /)
		if strings.HasPrefix(message, "/") {
			s.handleCommand(player, message)
			return
		}
		chatMsg := chat.Message{
			Text: "",
			Extra: []chat.Message{
				chat.Colored("<"+player.Username+"> ", "white"),
				chat.Text(message),
			},
		}
		log.Printf("<%s> %s", player.Username, message)
		s.broadcastChat(chatMsg)

	case 0x04: // Player Position
		x, _ := protocol.ReadFloat64(r)
		y, _ := protocol.ReadFloat64(r)
		z, _ := protocol.ReadFloat64(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		oldY := player.Y
		wasOnGround := player.OnGround
		player.X = x
		player.Y = y
		player.Z = z
		player.OnGround = onGround
		s.updateFallState(player, oldY, y, wasOnGround, onGround)
		player.mu.Unlock()
		s.broadcastEntityTeleport(player)
		s.sendChunkUpdates(player)

	case 0x05: // Player Look
		yaw, _ := protocol.ReadFloat32(r)
		pitch, _ := protocol.ReadFloat32(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.Yaw = yaw
		player.Pitch = pitch
		player.OnGround = onGround
		player.mu.Unlock()
		s.broadcastEntityLook(player)

	case 0x06: // Player Position And Look
		x, _ := protocol.ReadFloat64(r)
		y, _ := protocol.ReadFloat64(r)
		z, _ := protocol.ReadFloat64(r)
		yaw, _ := protocol.ReadFloat32(r)
		pitch, _ := protocol.ReadFloat32(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		oldY := player.Y
		wasOnGround := player.OnGround
		player.X = x
		player.Y = y
		player.Z = z
		player.Yaw = yaw
		player.Pitch = pitch
		player.OnGround = onGround
		s.updateFallState(player, oldY, y, wasOnGround, onGround)
		player.mu.Unlock()
		s.broadcastEntityTeleport(player)
		s.sendChunkUpdates(player)

	case 0x03: // Player (on ground)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.OnGround = onGround
		player.mu.Unlock()

	case 0x09: // Held Item Change
		slot, _ := protocol.ReadInt16(r)
		player.mu.Lock()
		player.ActiveSlot = slot
		player.mu.Unlock()
		// Inform other players about the newly selected held item so that
		// they see the correct item in this player's hand.
		s.broadcastHeldItem(player)

	case 0x07: // Player Digging
		s.handlePlayerDigging(player, r)

	case 0x0A: // Animation
		// Broadcast arm swing to other players
		s.broadcastAnimation(player, 0)

	case 0x15: // Client Settings
	// Ignore for now

	case 0x13: // Player Abilities (serverbound)
		// Client sends this when toggling flying or when F3+N is pressed to
		// cycle between Creative and Spectator.
		clientFlags, _ := protocol.ReadByte(r)
		_, _ = protocol.ReadFloat32(r) // Flying speed
		_, _ = protocol.ReadFloat32(r) // Walking speed

		player.mu.Lock()
		currentMode := player.GameMode
		player.mu.Unlock()

		// Detect F3+N gamemode toggle by checking the Instant Break flag (0x08).
		// Creative mode sets 0x08; Spectator does not.
		if currentMode == GameModeCreative && (clientFlags&0x08) == 0 {
			// Client removed Instant Break flag → switching to Spectator
			s.switchGameMode(player, GameModeSpectator)
		} else if currentMode == GameModeSpectator && (clientFlags&0x08) != 0 {
			// Client set Instant Break flag → switching to Creative
			s.switchGameMode(player, GameModeCreative)
		}

	case 0x14: // Tab-Complete
		text, err := protocol.ReadString(r)
		if err != nil {
			return
		}
		// We only support autocomplete for commands.
		if strings.HasPrefix(text, "/") {
			s.handleTabComplete(player, text)
		}

	case 0x17: // Plugin Message
	// Ignore for now

	case 0x02: // Use Entity
		targetID, _, err := protocol.ReadVarInt(r)
		if err != nil {
			return
		}
		useType, _, err := protocol.ReadVarInt(r)
		if err != nil {
			return
		}
		if useType == 1 { // Attack
			s.handleAttack(player, targetID)
		}

	case 0x16: // Client Status
		actionID, _, err := protocol.ReadVarInt(r)
		if err != nil {
			return
		}
		if actionID == 0 { // Perform Respawn
			s.handleRespawn(player)
		}

	case 0x0D: // Close Window
		s.handleCloseWindow(player, r)

	case 0x08: // Block Placement
		s.handleBlockPlacement(player, r)

	case 0x10: // Creative Inventory Action
		s.handleCreativeInventory(player, r)

	case 0x0E: // Click Window
		s.handleInventoryClick(player, r)
	}
}

// handlePlayerDigging processes Player Digging (0x07).
func (s *Server) handlePlayerDigging(player *Player, r *bytes.Reader) {
	status, _ := protocol.ReadByte(r)
	x, y, z, _ := protocol.ReadPosition(r)
	_, _ = protocol.ReadByte(r) // face
	if player.GameMode == GameModeSpectator {
		return // spectators can't interact
	}
	if status == 0 && player.GameMode == GameModeCreative {
		// Creative mode: instant break on start digging
		s.handleBlockBreak(player, x, y, z)
	} else if status == 2 {
		// Survival: finished digging
		s.handleBlockBreak(player, x, y, z)
	} else if status == 0 && player.GameMode == GameModeSurvival {
		// Survival: instant-break for zero-hardness blocks (torches, flowers, etc.)
		blockState := s.world.GetBlock(x, y, z)
		if world.IsInstantBreak(blockState >> 4) {
			s.handleBlockBreak(player, x, y, z)
		}
	} else if status == 3 || status == 4 {
		// Status 3 = drop item stack (Ctrl+Q), status 4 = drop single item (Q)
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		if player.Inventory[slotIndex].ItemID != -1 {
			dropItemID := player.Inventory[slotIndex].ItemID
			dropDamage := player.Inventory[slotIndex].Damage
			var dropCount byte = 1
			if status == 3 {
				// Ctrl+Q: drop entire stack
				dropCount = player.Inventory[slotIndex].Count
			}

			player.Inventory[slotIndex].Count -= dropCount
			if player.Inventory[slotIndex].Count <= 0 {
				player.Inventory[slotIndex] = Slot{ItemID: -1}
			}

			// Sync slot to client
			slot := player.Inventory[slotIndex]
			syncPkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
				protocol.WriteByte(w, 0) // Window ID 0 = player inventory
				protocol.WriteInt16(w, int16(slotIndex))
				protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
			})
			if player.Conn != nil {
				protocol.WritePacket(player.Conn, syncPkt)
			}

			px, py, pz := player.X, player.Y, player.Z
			f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
			f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
			f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
			f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)
			vx := -f1 * f4 * 0.3
			vy := -f3*0.3 + 0.1
			vz := f2 * f4 * 0.3

			player.mu.Unlock()
			s.SpawnItem(px, py+1.5, pz, vx, vy, vz, dropItemID, dropDamage, dropCount)
		} else {
			player.mu.Unlock()
		}
	}
}
