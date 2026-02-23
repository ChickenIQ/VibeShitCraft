package server

import (
	"testing"
	"github.com/StoreStation/VibeShitCraft/pkg/world"
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
