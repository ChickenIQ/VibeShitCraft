package server

import (
	"bytes"
	"sort"

	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
)

func (s *Server) sendSpawnChunks(player *Player) {
	playerChunkX := int32(int(player.X) >> 4)
	playerChunkZ := int32(int(player.Z) >> 4)

	player.mu.Lock()
	player.loadedChunks = make(map[ChunkPos]bool)
	player.lastChunkX = playerChunkX
	player.lastChunkZ = playerChunkZ

	var toQueue []ChunkPos
	for cx := playerChunkX - ViewDistance; cx <= playerChunkX+ViewDistance; cx++ {
		for cz := playerChunkZ - ViewDistance; cz <= playerChunkZ+ViewDistance; cz++ {
			pos := ChunkPos{cx, cz}
			player.loadedChunks[pos] = true
			toQueue = append(toQueue, pos)
		}
	}
	player.mu.Unlock()

	// Sort chunks by distance to player so nearest chunks arrive first
	sort.Slice(toQueue, func(i, j int) bool {
		dx1 := toQueue[i].X - playerChunkX
		dz1 := toQueue[i].Z - playerChunkZ
		dx2 := toQueue[j].X - playerChunkX
		dz2 := toQueue[j].Z - playerChunkZ
		return (dx1*dx1 + dz1*dz1) < (dx2*dx2 + dz2*dz2)
	})

	for _, pos := range toQueue {
		select {
		case player.ChunkQueue <- pos:
		default:
		}
	}
}

// sendChunkColumn generates and sends a single chunk column to a player.
// Returns true if the chunk was actually generated/sent.
func (s *Server) sendChunkColumn(player *Player, cx, cz int32) bool {
	pos := ChunkPos{cx, cz}
	player.mu.Lock()
	if !player.loadedChunks[pos] {
		// Player moved away before we could process the queued chunk
		player.mu.Unlock()
		return false
	}
	player.mu.Unlock()

	chunkData, primaryBitMask := s.world.GetChunkData(cx, cz)
	pkt := protocol.MarshalPacket(0x21, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, cx)                     // Chunk X
		protocol.WriteInt32(w, cz)                     // Chunk Z
		protocol.WriteBool(w, true)                    // Ground-up continuous
		protocol.WriteUint16(w, primaryBitMask)        // Primary bit mask
		protocol.WriteVarInt(w, int32(len(chunkData))) // Size
		w.Write(chunkData)                             // Data
	})

	player.mu.Lock()
	conn := player.Conn
	stillLoaded := player.loadedChunks[pos]
	player.mu.Unlock()

	if conn != nil && stillLoaded {
		protocol.WritePacket(conn, pkt)
	}
	return true
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

	var toQueue []ChunkPos
	for cx := currentChunkX - ViewDistance; cx <= currentChunkX+ViewDistance; cx++ {
		for cz := currentChunkZ - ViewDistance; cz <= currentChunkZ+ViewDistance; cz++ {
			pos := ChunkPos{cx, cz}
			if !player.loadedChunks[pos] {
				player.loadedChunks[pos] = true
				toQueue = append(toQueue, pos)
			}
		}
	}

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

	// Sort new chunks by distance to player
	sort.Slice(toQueue, func(i, j int) bool {
		dx1 := toQueue[i].X - currentChunkX
		dz1 := toQueue[i].Z - currentChunkZ
		dx2 := toQueue[j].X - currentChunkX
		dz2 := toQueue[j].Z - currentChunkZ
		return (dx1*dx1 + dz1*dz1) < (dx2*dx2 + dz2*dz2)
	})

	for _, pos := range toQueue {
		select {
		case player.ChunkQueue <- pos:
		default:
		}
	}

	// Send unload packets (empty chunk with primary bit mask 0)
	player.mu.Lock()
	conn := player.Conn
	player.mu.Unlock()

	if conn != nil {
		for _, pos := range toUnload {
			pkt := protocol.MarshalPacket(0x21, func(w *bytes.Buffer) {
				protocol.WriteInt32(w, pos.X)
				protocol.WriteInt32(w, pos.Z)
				protocol.WriteBool(w, true) // Ground-up continuous
				protocol.WriteUint16(w, 0)  // Primary bit mask: 0 = unload
				protocol.WriteVarInt(w, 0)  // Size: 0
			})
			protocol.WritePacket(conn, pkt)
		}
	}

	// Update entity tracking (e.g. spawn players who just came into range)
	s.updateEntityTracking(player)
}
