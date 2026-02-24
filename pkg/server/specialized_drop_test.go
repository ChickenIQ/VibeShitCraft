package server

import (
	"testing"

	"github.com/VibeShit/VibeShitCraft/pkg/world"
)

func TestSpecializedDrops(t *testing.T) {
	// 1. Test Wheat
	// Fully grown wheat (metadata 7) -> Wheat (296)
	itemID, damage, count := world.BlockToItemID(59<<4 | 7)
	if itemID != 296 || count != 1 {
		t.Errorf("Full wheat drop failed: got (%d, %d, %d), want (296, 0, 1)", itemID, damage, count)
	}
	// Small wheat (metadata 0) -> Seeds (295)
	itemID, damage, count = world.BlockToItemID(59<<4 | 0)
	if itemID != 295 || count != 1 {
		t.Errorf("Small wheat drop failed: got (%d, %d, %d), want (295, 0, 1)", itemID, damage, count)
	}

	// 2. Test Stairs
	// Cobblestone stairs (ID 67), metadata 4 (upside down)
	itemID, damage, count = world.BlockToItemID(67<<4 | 4)
	if itemID != 67 || damage != 0 {
		t.Errorf("Stair drop failed: got (%d, %d, %d), want (67, 0, 1)", itemID, damage, count)
	}

	// 3. Test Doors
	// Oak door bottom half (ID 64, metadata 0) -> Oak door item (324)
	itemID, damage, count = world.BlockToItemID(64<<4 | 0)
	if itemID != 324 || count != 1 {
		t.Errorf("Door bottom drop failed: got (%d, %d, %d), want (324, 0, 1)", itemID, damage, count)
	}
	// Oak door upper half (ID 64, metadata 8) -> Nothing (-1)
	itemID, damage, count = world.BlockToItemID(64<<4 | 8)
	if itemID != -1 {
		t.Errorf("Door top drop failed: got %d, want -1", itemID)
	}
}

func TestStoneVariantDrops(t *testing.T) {
	// Stone (meta 0) -> Cobblestone
	itemID, damage, _ := world.BlockToItemID(1<<4 | 0)
	if itemID != 4 || damage != 0 {
		t.Errorf("Stone drop: got (%d, %d), want (4, 0)", itemID, damage)
	}
	// Granite (meta 1) -> Granite
	itemID, damage, _ = world.BlockToItemID(1<<4 | 1)
	if itemID != 1 || damage != 1 {
		t.Errorf("Granite drop: got (%d, %d), want (1, 1)", itemID, damage)
	}
	// Diorite (meta 3) -> Diorite
	itemID, damage, _ = world.BlockToItemID(1<<4 | 3)
	if itemID != 1 || damage != 3 {
		t.Errorf("Diorite drop: got (%d, %d), want (1, 3)", itemID, damage)
	}
	// Andesite (meta 5) -> Andesite
	itemID, damage, _ = world.BlockToItemID(1<<4 | 5)
	if itemID != 1 || damage != 5 {
		t.Errorf("Andesite drop: got (%d, %d), want (1, 5)", itemID, damage)
	}
}

func TestOrientedBlocksDropBaseDamage(t *testing.T) {
	// These blocks use metadata for orientation and should drop with damage 0
	tests := []struct {
		name    string
		blockID uint16
		meta    uint16
	}{
		{"lever facing east", 69, 1},
		{"lever facing south", 69, 3},
		{"stone button facing east", 77, 1},
		{"stone button facing west", 77, 2},
		{"wooden button facing north", 143, 4},
		{"dispenser facing down", 23, 0},
		{"dispenser facing east", 23, 5},
		{"dropper facing north", 158, 2},
		{"piston facing up", 33, 1},
		{"hopper facing south", 154, 3},
		{"fence gate open", 107, 4},
		{"trapdoor open south", 96, 3},
		{"ladder facing north", 65, 2},
		{"pumpkin facing east", 86, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockState := tt.blockID<<4 | tt.meta
			itemID, damage, _ := world.BlockToItemID(blockState)
			if damage != 0 {
				t.Errorf("%s: got damage %d, want 0 (itemID=%d)", tt.name, damage, itemID)
			}
		})
	}
}

func TestVariantBlocksPreserveDamage(t *testing.T) {
	// These blocks use metadata for variants and should preserve it
	tests := []struct {
		name       string
		blockID    uint16
		meta       uint16
		wantID     int16
		wantDamage int16
	}{
		{"white wool", 35, 0, 35, 0},
		{"orange wool", 35, 1, 35, 1},
		{"blue wool", 35, 11, 35, 11},
		{"red stained clay", 159, 14, 159, 14},
		{"lime carpet", 171, 5, 171, 5},
		{"sandstone", 24, 0, 24, 0},
		{"chiseled sandstone", 24, 1, 24, 1},
		{"smooth sandstone", 24, 2, 24, 2},
		{"stone bricks", 98, 0, 98, 0},
		{"mossy stone bricks", 98, 1, 98, 1},
		{"cracked stone bricks", 98, 2, 98, 2},
		{"prismarine", 168, 0, 168, 0},
		{"prismarine bricks", 168, 1, 168, 1},
		{"dark prismarine", 168, 2, 168, 2},
		{"red sand", 12, 1, 12, 1},
		{"cobblestone wall", 139, 0, 139, 0},
		{"mossy cobblestone wall", 139, 1, 139, 1},
		{"wet sponge", 19, 1, 19, 1},
		{"dry sponge", 19, 0, 19, 0},
		{"red sandstone", 179, 0, 179, 0},
		{"chiseled red sandstone", 179, 1, 179, 1},
		{"poppy", 38, 0, 38, 0},
		{"blue orchid", 38, 1, 38, 1},
		{"oak sapling", 6, 0, 6, 0},
		{"spruce sapling", 6, 1, 6, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockState := tt.blockID<<4 | tt.meta
			itemID, damage, _ := world.BlockToItemID(blockState)
			if itemID != tt.wantID || damage != tt.wantDamage {
				t.Errorf("%s: got (%d, %d), want (%d, %d)", tt.name, itemID, damage, tt.wantID, tt.wantDamage)
			}
		})
	}
}

func TestSlabDrops(t *testing.T) {
	// Stone slab bottom half (meta 0) -> stone slab, damage 0
	itemID, damage, count := world.BlockToItemID(44<<4 | 0)
	if itemID != 44 || damage != 0 || count != 1 {
		t.Errorf("Stone slab bottom: got (%d, %d, %d), want (44, 0, 1)", itemID, damage, count)
	}
	// Stone slab upper half (meta 8) -> stone slab, damage 0 (strip upper bit)
	itemID, damage, count = world.BlockToItemID(44<<4 | 8)
	if itemID != 44 || damage != 0 || count != 1 {
		t.Errorf("Stone slab upper: got (%d, %d, %d), want (44, 0, 1)", itemID, damage, count)
	}
	// Sandstone slab upper half (meta 9 = type 1 + upper bit 8) -> slab damage 1
	itemID, damage, count = world.BlockToItemID(44<<4 | 9)
	if itemID != 44 || damage != 1 || count != 1 {
		t.Errorf("Sandstone slab upper: got (%d, %d, %d), want (44, 1, 1)", itemID, damage, count)
	}
	// Double stone slab -> drops 2 stone slabs
	itemID, damage, count = world.BlockToItemID(43<<4 | 0)
	if itemID != 44 || damage != 0 || count != 2 {
		t.Errorf("Double stone slab: got (%d, %d, %d), want (44, 0, 2)", itemID, damage, count)
	}
	// Wooden slab upper half (meta 8) -> wooden slab damage 0
	itemID, damage, count = world.BlockToItemID(126<<4 | 8)
	if itemID != 126 || damage != 0 || count != 1 {
		t.Errorf("Wood slab upper: got (%d, %d, %d), want (126, 0, 1)", itemID, damage, count)
	}
	// Double wooden slab (spruce, meta 1) -> 2 wooden slabs damage 1
	itemID, damage, count = world.BlockToItemID(125<<4 | 1)
	if itemID != 126 || damage != 1 || count != 2 {
		t.Errorf("Double wood slab: got (%d, %d, %d), want (126, 1, 2)", itemID, damage, count)
	}
}

func TestSpecialBlockDrops(t *testing.T) {
	// Sapling with age bit set (meta 8+type) -> strip age bit
	itemID, damage, _ := world.BlockToItemID(6<<4 | 9) // spruce sapling aged (1 | 8)
	if itemID != 6 || damage != 1 {
		t.Errorf("Aged spruce sapling: got (%d, %d), want (6, 1)", itemID, damage)
	}

	// Logs strip orientation, keep type
	itemID, damage, _ = world.BlockToItemID(17<<4 | 5) // spruce log east-west (1 | 4)
	if itemID != 17 || damage != 1 {
		t.Errorf("Rotated spruce log: got (%d, %d), want (17, 1)", itemID, damage)
	}

	// Anvil: meta = rotation(0-3) | damage_level(0-2)<<2
	// Slightly damaged anvil facing east: rotation=1, damage=1 -> meta = 1 | (1<<2) = 5
	itemID, damage, _ = world.BlockToItemID(145<<4 | 5)
	if itemID != 145 || damage != 1 {
		t.Errorf("Damaged anvil: got (%d, %d), want (145, 1)", itemID, damage)
	}

	// Furnace with direction metadata -> damage 0
	itemID, damage, _ = world.BlockToItemID(61<<4 | 3)
	if itemID != 61 || damage != 0 {
		t.Errorf("Furnace: got (%d, %d), want (61, 0)", itemID, damage)
	}

	// Lit furnace -> drops furnace (61)
	itemID, damage, _ = world.BlockToItemID(62<<4 | 2)
	if itemID != 61 || damage != 0 {
		t.Errorf("Lit furnace: got (%d, %d), want (61, 0)", itemID, damage)
	}

	// Bed head part -> nothing
	itemID, _, _ = world.BlockToItemID(26<<4 | 8)
	if itemID != -1 {
		t.Errorf("Bed head: got itemID %d, want -1", itemID)
	}
	// Bed foot part -> bed item (355)
	itemID, _, _ = world.BlockToItemID(26<<4 | 0)
	if itemID != 355 {
		t.Errorf("Bed foot: got itemID %d, want 355", itemID)
	}

	// Quartz pillar (meta 3 = east-west pillar) -> drops as quartz pillar (meta 2)
	itemID, damage, _ = world.BlockToItemID(155<<4 | 3)
	if itemID != 155 || damage != 2 {
		t.Errorf("Quartz pillar E-W: got (%d, %d), want (155, 2)", itemID, damage)
	}
	// Chiseled quartz (meta 1) -> keeps meta 1
	itemID, damage, _ = world.BlockToItemID(155<<4 | 1)
	if itemID != 155 || damage != 1 {
		t.Errorf("Chiseled quartz: got (%d, %d), want (155, 1)", itemID, damage)
	}

	// Coarse dirt (meta 1) -> drops itself
	itemID, damage, _ = world.BlockToItemID(3<<4 | 1)
	if itemID != 3 || damage != 1 {
		t.Errorf("Coarse dirt: got (%d, %d), want (3, 1)", itemID, damage)
	}

	// Stained glass pane -> drops nothing
	itemID, _, _ = world.BlockToItemID(160<<4 | 5)
	if itemID != -1 {
		t.Errorf("Stained glass pane: got itemID %d, want -1", itemID)
	}
}

func TestDefaultDropsBaseDamage(t *testing.T) {
	// Any unhandled block with metadata should drop with damage 0
	// Use block ID 22 (lapis block) as example - not explicitly handled
	itemID, damage, count := world.BlockToItemID(22<<4 | 0)
	if itemID != 22 || damage != 0 || count != 1 {
		t.Errorf("Lapis block: got (%d, %d, %d), want (22, 0, 1)", itemID, damage, count)
	}

	// Iron block (42) with some spurious metadata -> should still drop with damage 0
	itemID, damage, count = world.BlockToItemID(42<<4 | 3)
	if itemID != 42 || damage != 0 || count != 1 {
		t.Errorf("Iron block with metadata: got (%d, %d, %d), want (42, 0, 1)", itemID, damage, count)
	}
}
