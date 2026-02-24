package server

import (
	"bytes"

	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
)

func (s *Server) sendSpawnChunks(player *Player) {
	playerChunkX := int32(int(player.X) >> 4)
	playerChunkZ := int32(int(player.Z) >> 4)

	player.mu.Lock()
	player.loadedChunks = make(map[ChunkPos]bool)
	player.lastChunkX = playerChunkX
	player.lastChunkZ = playerChunkZ
	player.mu.Unlock()

	// Send chunks in a square around the player's chunk
	for cx := playerChunkX - ViewDistance; cx <= playerChunkX+ViewDistance; cx++ {
		for cz := playerChunkZ - ViewDistance; cz <= playerChunkZ+ViewDistance; cz++ {
			s.sendChunkColumn(player, cx, cz)
		}
	}
}

// sendChunkColumn generates and sends a single chunk column to a player.
func (s *Server) sendChunkColumn(player *Player, cx, cz int32) {
	pos := ChunkPos{cx, cz}
	player.mu.Lock()
	if player.loadedChunks[pos] {
		player.mu.Unlock()
		return
	}
	player.loadedChunks[pos] = true
	player.mu.Unlock()

	chunkData, primaryBitMask := s.world.GetChunkData(int32(cx), int32(cz))
	pkt := protocol.MarshalPacket(0x21, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, cx)                     // Chunk X
		protocol.WriteInt32(w, cz)                     // Chunk Z
		protocol.WriteBool(w, true)                    // Ground-up continuous
		protocol.WriteUint16(w, primaryBitMask)        // Primary bit mask
		protocol.WriteVarInt(w, int32(len(chunkData))) // Size
		w.Write(chunkData)                             // Data
	})
	player.mu.Lock()
	if player.Conn != nil {
		protocol.WritePacket(player.Conn, pkt)
	}
	player.mu.Unlock()
}

// sendChunkUpdates streams new chunks to the player when they cross chunk boundaries
// and unloads chunks that are too far away.
func (s *Server) sendChunkUpdates(player *Player) {
	player.mu.Lock()
	currentChunkX := int32(int(player.X) >> 4)
	currentChunkZ := int32(int(player.Z) >> 4)

	if currentChunkX == player.lastChunkX && currentChunkZ == player.lastChunkZ {
		player.mu.Unlock()
		return
	}

	player.lastChunkX = currentChunkX
	player.lastChunkZ = currentChunkZ
	player.mu.Unlock()

	// Send new chunks that are now in range
	for cx := currentChunkX - ViewDistance; cx <= currentChunkX+ViewDistance; cx++ {
		for cz := currentChunkZ - ViewDistance; cz <= currentChunkZ+ViewDistance; cz++ {
			s.sendChunkColumn(player, cx, cz)
		}
	}

	// Unload chunks that are now out of range
	player.mu.Lock()
	var toUnload []ChunkPos
	for pos := range player.loadedChunks {
		dx := pos.X - currentChunkX
		dz := pos.Z - currentChunkZ
		if dx < -ViewDistance || dx > ViewDistance || dz < -ViewDistance || dz > ViewDistance {
			toUnload = append(toUnload, pos)
		}
	}
	for _, pos := range toUnload {
		delete(player.loadedChunks, pos)
	}
	player.mu.Unlock()

	// Send unload packets (empty chunk with primary bit mask 0)
	for _, pos := range toUnload {
		pkt := protocol.MarshalPacket(0x21, func(w *bytes.Buffer) {
			protocol.WriteInt32(w, pos.X)
			protocol.WriteInt32(w, pos.Z)
			protocol.WriteBool(w, true) // Ground-up continuous
			protocol.WriteUint16(w, 0)  // Primary bit mask: 0 = unload
			protocol.WriteVarInt(w, 0)  // Size: 0
		})
		player.mu.Lock()
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
	}

	// Update entity tracking (e.g. spawn players who just came into range)
	s.updateEntityTracking(player)
}
