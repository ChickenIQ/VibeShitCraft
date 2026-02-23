package server

// Ingredient represents a required item in a crafting recipe slot.
type Ingredient struct {
	ID     int16 // Required item ID, -1 for empty slot
	Damage int16 // Required damage value, -1 for any
}

// CraftingRecipe represents a shaped crafting recipe.
type CraftingRecipe struct {
	Width        int
	Height       int
	Ingredients  []Ingredient
	ResultID     int16
	ResultCount  byte
	ResultDamage int16
}

// craftingRecipes defines all available crafting recipes.
var craftingRecipes = []CraftingRecipe{
	// ===== Planks from logs =====
	{1, 1, []Ingredient{{17, 0}}, 5, 4, 0},  // Oak Log → Oak Planks
	{1, 1, []Ingredient{{17, 1}}, 5, 4, 1},  // Spruce Log → Spruce Planks
	{1, 1, []Ingredient{{17, 2}}, 5, 4, 2},  // Birch Log → Birch Planks
	{1, 1, []Ingredient{{17, 3}}, 5, 4, 3},  // Jungle Log → Jungle Planks
	{1, 1, []Ingredient{{162, 0}}, 5, 4, 4}, // Acacia Log → Acacia Planks
	{1, 1, []Ingredient{{162, 1}}, 5, 4, 5}, // Dark Oak Log → Dark Oak Planks

	// ===== Sticks =====
	{1, 2, []Ingredient{{5, -1}, {5, -1}}, 280, 4, 0},

	// ===== Crafting Table =====
	{2, 2, []Ingredient{{5, -1}, {5, -1}, {5, -1}, {5, -1}}, 58, 1, 0},

	// ===== Torch =====
	{1, 2, []Ingredient{{263, -1}, {280, -1}}, 50, 4, 0},

	// ===== Furnace =====
	{3, 3, []Ingredient{
		{4, -1}, {4, -1}, {4, -1},
		{4, -1}, {-1, 0}, {4, -1},
		{4, -1}, {4, -1}, {4, -1},
	}, 61, 1, 0},

	// ===== Chest =====
	{3, 3, []Ingredient{
		{5, -1}, {5, -1}, {5, -1},
		{5, -1}, {-1, 0}, {5, -1},
		{5, -1}, {5, -1}, {5, -1},
	}, 54, 1, 0},

	// ===== Wooden Tools =====
	// Wooden Pickaxe
	{3, 3, []Ingredient{
		{5, -1}, {5, -1}, {5, -1},
		{-1, 0}, {280, -1}, {-1, 0},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 270, 1, 0},
	// Wooden Axe (left)
	{2, 3, []Ingredient{
		{5, -1}, {5, -1},
		{5, -1}, {280, -1},
		{-1, 0}, {280, -1},
	}, 271, 1, 0},
	// Wooden Axe (right/mirrored)
	{2, 3, []Ingredient{
		{5, -1}, {5, -1},
		{280, -1}, {5, -1},
		{280, -1}, {-1, 0},
	}, 271, 1, 0},
	// Wooden Shovel
	{1, 3, []Ingredient{{5, -1}, {280, -1}, {280, -1}}, 269, 1, 0},
	// Wooden Sword
	{1, 3, []Ingredient{{5, -1}, {5, -1}, {280, -1}}, 268, 1, 0},
	// Wooden Hoe (left)
	{2, 3, []Ingredient{
		{5, -1}, {5, -1},
		{-1, 0}, {280, -1},
		{-1, 0}, {280, -1},
	}, 290, 1, 0},
	// Wooden Hoe (right)
	{2, 3, []Ingredient{
		{5, -1}, {5, -1},
		{280, -1}, {-1, 0},
		{280, -1}, {-1, 0},
	}, 290, 1, 0},

	// ===== Stone Tools =====
	// Stone Pickaxe
	{3, 3, []Ingredient{
		{4, -1}, {4, -1}, {4, -1},
		{-1, 0}, {280, -1}, {-1, 0},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 274, 1, 0},
	// Stone Axe (left)
	{2, 3, []Ingredient{
		{4, -1}, {4, -1},
		{4, -1}, {280, -1},
		{-1, 0}, {280, -1},
	}, 275, 1, 0},
	// Stone Axe (right)
	{2, 3, []Ingredient{
		{4, -1}, {4, -1},
		{280, -1}, {4, -1},
		{280, -1}, {-1, 0},
	}, 275, 1, 0},
	// Stone Shovel
	{1, 3, []Ingredient{{4, -1}, {280, -1}, {280, -1}}, 273, 1, 0},
	// Stone Sword
	{1, 3, []Ingredient{{4, -1}, {4, -1}, {280, -1}}, 272, 1, 0},
	// Stone Hoe (left)
	{2, 3, []Ingredient{
		{4, -1}, {4, -1},
		{-1, 0}, {280, -1},
		{-1, 0}, {280, -1},
	}, 291, 1, 0},
	// Stone Hoe (right)
	{2, 3, []Ingredient{
		{4, -1}, {4, -1},
		{280, -1}, {-1, 0},
		{280, -1}, {-1, 0},
	}, 291, 1, 0},

	// ===== Iron Tools =====
	// Iron Pickaxe
	{3, 3, []Ingredient{
		{265, -1}, {265, -1}, {265, -1},
		{-1, 0}, {280, -1}, {-1, 0},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 257, 1, 0},
	// Iron Axe (left)
	{2, 3, []Ingredient{
		{265, -1}, {265, -1},
		{265, -1}, {280, -1},
		{-1, 0}, {280, -1},
	}, 258, 1, 0},
	// Iron Axe (right)
	{2, 3, []Ingredient{
		{265, -1}, {265, -1},
		{280, -1}, {265, -1},
		{280, -1}, {-1, 0},
	}, 258, 1, 0},
	// Iron Shovel
	{1, 3, []Ingredient{{265, -1}, {280, -1}, {280, -1}}, 256, 1, 0},
	// Iron Sword
	{1, 3, []Ingredient{{265, -1}, {265, -1}, {280, -1}}, 267, 1, 0},
	// Iron Hoe (left)
	{2, 3, []Ingredient{
		{265, -1}, {265, -1},
		{-1, 0}, {280, -1},
		{-1, 0}, {280, -1},
	}, 292, 1, 0},
	// Iron Hoe (right)
	{2, 3, []Ingredient{
		{265, -1}, {265, -1},
		{280, -1}, {-1, 0},
		{280, -1}, {-1, 0},
	}, 292, 1, 0},

	// ===== Diamond Tools =====
	// Diamond Pickaxe
	{3, 3, []Ingredient{
		{264, -1}, {264, -1}, {264, -1},
		{-1, 0}, {280, -1}, {-1, 0},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 278, 1, 0},
	// Diamond Axe (left)
	{2, 3, []Ingredient{
		{264, -1}, {264, -1},
		{264, -1}, {280, -1},
		{-1, 0}, {280, -1},
	}, 279, 1, 0},
	// Diamond Axe (right)
	{2, 3, []Ingredient{
		{264, -1}, {264, -1},
		{280, -1}, {264, -1},
		{280, -1}, {-1, 0},
	}, 279, 1, 0},
	// Diamond Shovel
	{1, 3, []Ingredient{{264, -1}, {280, -1}, {280, -1}}, 277, 1, 0},
	// Diamond Sword
	{1, 3, []Ingredient{{264, -1}, {264, -1}, {280, -1}}, 276, 1, 0},
	// Diamond Hoe (left)
	{2, 3, []Ingredient{
		{264, -1}, {264, -1},
		{-1, 0}, {280, -1},
		{-1, 0}, {280, -1},
	}, 293, 1, 0},
	// Diamond Hoe (right)
	{2, 3, []Ingredient{
		{264, -1}, {264, -1},
		{280, -1}, {-1, 0},
		{280, -1}, {-1, 0},
	}, 293, 1, 0},

	// ===== Gold Tools =====
	// Gold Pickaxe
	{3, 3, []Ingredient{
		{266, -1}, {266, -1}, {266, -1},
		{-1, 0}, {280, -1}, {-1, 0},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 285, 1, 0},
	// Gold Axe (left)
	{2, 3, []Ingredient{
		{266, -1}, {266, -1},
		{266, -1}, {280, -1},
		{-1, 0}, {280, -1},
	}, 286, 1, 0},
	// Gold Axe (right)
	{2, 3, []Ingredient{
		{266, -1}, {266, -1},
		{280, -1}, {266, -1},
		{280, -1}, {-1, 0},
	}, 286, 1, 0},
	// Gold Shovel
	{1, 3, []Ingredient{{266, -1}, {280, -1}, {280, -1}}, 284, 1, 0},
	// Gold Sword
	{1, 3, []Ingredient{{266, -1}, {266, -1}, {280, -1}}, 283, 1, 0},
	// Gold Hoe (left)
	{2, 3, []Ingredient{
		{266, -1}, {266, -1},
		{-1, 0}, {280, -1},
		{-1, 0}, {280, -1},
	}, 294, 1, 0},
	// Gold Hoe (right)
	{2, 3, []Ingredient{
		{266, -1}, {266, -1},
		{280, -1}, {-1, 0},
		{280, -1}, {-1, 0},
	}, 294, 1, 0},

	// ===== Bread =====
	{3, 1, []Ingredient{{296, -1}, {296, -1}, {296, -1}}, 297, 1, 0},

	// ===== Ladder =====
	{3, 3, []Ingredient{
		{280, -1}, {-1, 0}, {280, -1},
		{280, -1}, {280, -1}, {280, -1},
		{280, -1}, {-1, 0}, {280, -1},
	}, 65, 3, 0},

	// ===== Bowl =====
	{3, 2, []Ingredient{
		{5, -1}, {-1, 0}, {5, -1},
		{-1, 0}, {5, -1}, {-1, 0},
	}, 281, 4, 0},

	// ===== Bucket =====
	{3, 2, []Ingredient{
		{265, -1}, {-1, 0}, {265, -1},
		{-1, 0}, {265, -1}, {-1, 0},
	}, 325, 1, 0},

	// ===== Arrow =====
	{1, 3, []Ingredient{{318, -1}, {280, -1}, {288, -1}}, 262, 4, 0},

	// ===== Sign =====
	{3, 3, []Ingredient{
		{5, -1}, {5, -1}, {5, -1},
		{5, -1}, {5, -1}, {5, -1},
		{-1, 0}, {280, -1}, {-1, 0},
	}, 323, 3, 0},

	// ===== Oak Door =====
	{2, 3, []Ingredient{
		{5, 0}, {5, 0},
		{5, 0}, {5, 0},
		{5, 0}, {5, 0},
	}, 324, 3, 0},

	// ===== Oak Fence =====
	{3, 2, []Ingredient{
		{5, 0}, {280, -1}, {5, 0},
		{5, 0}, {280, -1}, {5, 0},
	}, 85, 3, 0},

	// ===== Iron Armor =====
	// Iron Helmet
	{3, 2, []Ingredient{
		{265, -1}, {265, -1}, {265, -1},
		{265, -1}, {-1, 0}, {265, -1},
	}, 306, 1, 0},
	// Iron Chestplate
	{3, 3, []Ingredient{
		{265, -1}, {-1, 0}, {265, -1},
		{265, -1}, {265, -1}, {265, -1},
		{265, -1}, {265, -1}, {265, -1},
	}, 307, 1, 0},
	// Iron Leggings
	{3, 3, []Ingredient{
		{265, -1}, {265, -1}, {265, -1},
		{265, -1}, {-1, 0}, {265, -1},
		{265, -1}, {-1, 0}, {265, -1},
	}, 308, 1, 0},
	// Iron Boots
	{3, 2, []Ingredient{
		{265, -1}, {-1, 0}, {265, -1},
		{265, -1}, {-1, 0}, {265, -1},
	}, 309, 1, 0},

	// ===== Diamond Armor =====
	{3, 2, []Ingredient{
		{264, -1}, {264, -1}, {264, -1},
		{264, -1}, {-1, 0}, {264, -1},
	}, 310, 1, 0},
	{3, 3, []Ingredient{
		{264, -1}, {-1, 0}, {264, -1},
		{264, -1}, {264, -1}, {264, -1},
		{264, -1}, {264, -1}, {264, -1},
	}, 311, 1, 0},
	{3, 3, []Ingredient{
		{264, -1}, {264, -1}, {264, -1},
		{264, -1}, {-1, 0}, {264, -1},
		{264, -1}, {-1, 0}, {264, -1},
	}, 312, 1, 0},
	{3, 2, []Ingredient{
		{264, -1}, {-1, 0}, {264, -1},
		{264, -1}, {-1, 0}, {264, -1},
	}, 313, 1, 0},

	// ===== Gold Armor =====
	{3, 2, []Ingredient{
		{266, -1}, {266, -1}, {266, -1},
		{266, -1}, {-1, 0}, {266, -1},
	}, 314, 1, 0},
	{3, 3, []Ingredient{
		{266, -1}, {-1, 0}, {266, -1},
		{266, -1}, {266, -1}, {266, -1},
		{266, -1}, {266, -1}, {266, -1},
	}, 315, 1, 0},
	{3, 3, []Ingredient{
		{266, -1}, {266, -1}, {266, -1},
		{266, -1}, {-1, 0}, {266, -1},
		{266, -1}, {-1, 0}, {266, -1},
	}, 316, 1, 0},
	{3, 2, []Ingredient{
		{266, -1}, {-1, 0}, {266, -1},
		{266, -1}, {-1, 0}, {266, -1},
	}, 317, 1, 0},

	// ===== Leather Armor =====
	{3, 2, []Ingredient{
		{334, -1}, {334, -1}, {334, -1},
		{334, -1}, {-1, 0}, {334, -1},
	}, 298, 1, 0},
	{3, 3, []Ingredient{
		{334, -1}, {-1, 0}, {334, -1},
		{334, -1}, {334, -1}, {334, -1},
		{334, -1}, {334, -1}, {334, -1},
	}, 299, 1, 0},
	{3, 3, []Ingredient{
		{334, -1}, {334, -1}, {334, -1},
		{334, -1}, {-1, 0}, {334, -1},
		{334, -1}, {-1, 0}, {334, -1},
	}, 300, 1, 0},
	{3, 2, []Ingredient{
		{334, -1}, {-1, 0}, {334, -1},
		{334, -1}, {-1, 0}, {334, -1},
	}, 301, 1, 0},
}

// findRecipe checks the given crafting grid for a matching recipe.
// gridSize is 2 for the player inventory grid or 3 for the crafting table.
func findRecipe(grid []Slot, gridSize int) *CraftingRecipe {
	for i := range craftingRecipes {
		r := &craftingRecipes[i]
		if r.Width > gridSize || r.Height > gridSize {
			continue
		}
		for ox := 0; ox <= gridSize-r.Width; ox++ {
			for oy := 0; oy <= gridSize-r.Height; oy++ {
				if matchRecipeAt(grid, gridSize, r, ox, oy) {
					return r
				}
			}
		}
	}
	return nil
}

// matchRecipeAt checks if a recipe matches at the given offset in the grid.
func matchRecipeAt(grid []Slot, gridSize int, r *CraftingRecipe, ox, oy int) bool {
	for gy := 0; gy < gridSize; gy++ {
		for gx := 0; gx < gridSize; gx++ {
			actual := grid[gy*gridSize+gx]
			rx := gx - ox
			ry := gy - oy
			if rx >= 0 && rx < r.Width && ry >= 0 && ry < r.Height {
				expected := r.Ingredients[ry*r.Width+rx]
				if expected.ID == -1 {
					if actual.ItemID != -1 {
						return false
					}
				} else {
					if actual.ItemID != expected.ID {
						return false
					}
					if expected.Damage != -1 && actual.Damage != expected.Damage {
						return false
					}
				}
			} else {
				// Outside recipe pattern: grid cell must be empty
				if actual.ItemID != -1 {
					return false
				}
			}
		}
	}
	return true
}

// updateCraftOutput2x2 checks the player's 2x2 crafting grid (Inventory[1-4])
// and sets the crafting output (Inventory[0]) based on matching recipes.
// Must be called with player.mu held.
func updateCraftOutput2x2(player *Player) {
	grid := make([]Slot, 4)
	copy(grid, player.Inventory[1:5])
	recipe := findRecipe(grid, 2)
	if recipe != nil {
		player.Inventory[0] = Slot{
			ItemID: recipe.ResultID,
			Count:  recipe.ResultCount,
			Damage: recipe.ResultDamage,
		}
	} else {
		player.Inventory[0] = Slot{ItemID: -1}
	}
}

// consumeCraftIngredients2x2 decrements each non-empty ingredient in the 2x2 grid by 1.
// Must be called with player.mu held.
func consumeCraftIngredients2x2(player *Player) {
	for i := 1; i <= 4; i++ {
		if player.Inventory[i].ItemID != -1 {
			player.Inventory[i].Count--
			if player.Inventory[i].Count <= 0 {
				player.Inventory[i] = Slot{ItemID: -1}
			}
		}
	}
}

// updateCraftOutput3x3 checks the player's 3x3 crafting grid (CraftTableGrid)
// and sets the crafting output (CraftTableOutput) based on matching recipes.
// Must be called with player.mu held.
func updateCraftOutput3x3(player *Player) {
	grid := player.CraftTableGrid[:]
	recipe := findRecipe(grid, 3)
	if recipe != nil {
		player.CraftTableOutput = Slot{
			ItemID: recipe.ResultID,
			Count:  recipe.ResultCount,
			Damage: recipe.ResultDamage,
		}
	} else {
		player.CraftTableOutput = Slot{ItemID: -1}
	}
}

// consumeCraftIngredients3x3 decrements each non-empty ingredient in the 3x3 grid by 1.
// Must be called with player.mu held.
func consumeCraftIngredients3x3(player *Player) {
	for i := 0; i < 9; i++ {
		if player.CraftTableGrid[i].ItemID != -1 {
			player.CraftTableGrid[i].Count--
			if player.CraftTableGrid[i].Count <= 0 {
				player.CraftTableGrid[i] = Slot{ItemID: -1}
			}
		}
	}
}
