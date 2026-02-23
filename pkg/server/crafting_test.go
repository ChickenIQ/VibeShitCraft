package server

import "testing"

func TestFindRecipePlanks(t *testing.T) {
	// Oak log in top-left of 2x2 grid → 4 oak planks
	grid := []Slot{
		{ItemID: 17, Count: 1, Damage: 0},
		{ItemID: -1},
		{ItemID: -1},
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for oak log → planks")
	}
	if r.ResultID != 5 || r.ResultCount != 4 || r.ResultDamage != 0 {
		t.Errorf("expected 4x planks (5:0), got %d:%d x%d", r.ResultID, r.ResultDamage, r.ResultCount)
	}
}

func TestFindRecipePlanksBottomRight(t *testing.T) {
	// Oak log in bottom-right of 2x2 grid → should still match
	grid := []Slot{
		{ItemID: -1},
		{ItemID: -1},
		{ItemID: -1},
		{ItemID: 17, Count: 1, Damage: 0},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for oak log in bottom-right")
	}
	if r.ResultID != 5 {
		t.Errorf("expected planks (5), got %d", r.ResultID)
	}
}

func TestFindRecipeSprucePlanks(t *testing.T) {
	// Spruce log → spruce planks
	grid := []Slot{
		{ItemID: -1},
		{ItemID: 17, Count: 1, Damage: 1},
		{ItemID: -1},
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for spruce log → planks")
	}
	if r.ResultID != 5 || r.ResultDamage != 1 {
		t.Errorf("expected spruce planks (5:1), got %d:%d", r.ResultID, r.ResultDamage)
	}
}

func TestFindRecipeSticks(t *testing.T) {
	// Two planks vertically in left column → 4 sticks
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1},
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for sticks")
	}
	if r.ResultID != 280 || r.ResultCount != 4 {
		t.Errorf("expected 4x sticks (280), got %d x%d", r.ResultID, r.ResultCount)
	}
}

func TestFindRecipeSticksRightColumn(t *testing.T) {
	// Two planks vertically in right column → 4 sticks
	grid := []Slot{
		{ItemID: -1},
		{ItemID: 5, Count: 1, Damage: 2}, // birch planks
		{ItemID: -1},
		{ItemID: 5, Count: 1, Damage: 2},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for sticks in right column")
	}
	if r.ResultID != 280 {
		t.Errorf("expected sticks (280), got %d", r.ResultID)
	}
}

func TestFindRecipeNoMatchHorizontalPlanks(t *testing.T) {
	// Two planks horizontally should NOT match sticks (which requires vertical)
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1},
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	// This should NOT match sticks. It could match something else or nothing.
	if r != nil && r.ResultID == 280 {
		t.Error("horizontal planks should not match sticks recipe")
	}
}

func TestFindRecipeCraftingTable(t *testing.T) {
	// 4 planks in 2x2 → crafting table
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: 5, Count: 1, Damage: 0},
		{ItemID: 5, Count: 1, Damage: 0},
	}
	r := findRecipe(grid, 2)
	if r == nil {
		t.Fatal("expected recipe match for crafting table")
	}
	if r.ResultID != 58 || r.ResultCount != 1 {
		t.Errorf("expected 1x crafting table (58), got %d x%d", r.ResultID, r.ResultCount)
	}
}

func TestFindRecipeNoMatch(t *testing.T) {
	// Random items that don't form a recipe
	grid := []Slot{
		{ItemID: 3, Count: 1, Damage: 0}, // dirt
		{ItemID: -1},
		{ItemID: 1, Count: 1, Damage: 0}, // stone
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	if r != nil {
		t.Errorf("expected no recipe match, got %d", r.ResultID)
	}
}

func TestFindRecipeEmptyGrid(t *testing.T) {
	grid := []Slot{
		{ItemID: -1},
		{ItemID: -1},
		{ItemID: -1},
		{ItemID: -1},
	}
	r := findRecipe(grid, 2)
	if r != nil {
		t.Error("expected no recipe match for empty grid")
	}
}

func TestFindRecipe3x3Furnace(t *testing.T) {
	// Furnace: ring of cobblestone in 3x3
	grid := []Slot{
		{ItemID: 4, Count: 1, Damage: 0}, {ItemID: 4, Count: 1, Damage: 0}, {ItemID: 4, Count: 1, Damage: 0},
		{ItemID: 4, Count: 1, Damage: 0}, {ItemID: -1},                      {ItemID: 4, Count: 1, Damage: 0},
		{ItemID: 4, Count: 1, Damage: 0}, {ItemID: 4, Count: 1, Damage: 0}, {ItemID: 4, Count: 1, Damage: 0},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for furnace")
	}
	if r.ResultID != 61 {
		t.Errorf("expected furnace (61), got %d", r.ResultID)
	}
}

func TestFindRecipe3x3WoodenPickaxe(t *testing.T) {
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1},                      {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: -1},                      {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for wooden pickaxe")
	}
	if r.ResultID != 270 {
		t.Errorf("expected wooden pickaxe (270), got %d", r.ResultID)
	}
}

func TestFindRecipe3x3Torch(t *testing.T) {
	// Torch in 3x3 grid (should match in any column)
	grid := []Slot{
		{ItemID: -1}, {ItemID: 263, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: -1}, {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: -1}, {ItemID: -1},                        {ItemID: -1},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for torch in 3x3 grid")
	}
	if r.ResultID != 50 || r.ResultCount != 4 {
		t.Errorf("expected 4x torch (50), got %d x%d", r.ResultID, r.ResultCount)
	}
}

func TestFindRecipe2x2RecipeIn3x3Grid(t *testing.T) {
	// Crafting table recipe (2x2 planks) placed in top-left of 3x3 grid
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: -1},                      {ItemID: -1},                      {ItemID: -1},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for crafting table in 3x3 grid")
	}
	if r.ResultID != 58 {
		t.Errorf("expected crafting table (58), got %d", r.ResultID)
	}
}

func TestUpdateCraftOutput2x2(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	// Place oak log in slot 1
	player.Inventory[1] = Slot{ItemID: 17, Count: 1, Damage: 0}
	updateCraftOutput2x2(player)
	if player.Inventory[0].ItemID != 5 || player.Inventory[0].Count != 4 {
		t.Errorf("expected 4x planks in output, got %d x%d", player.Inventory[0].ItemID, player.Inventory[0].Count)
	}
}

func TestUpdateCraftOutput2x2Empty(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	updateCraftOutput2x2(player)
	if player.Inventory[0].ItemID != -1 {
		t.Errorf("expected empty output for empty grid, got %d", player.Inventory[0].ItemID)
	}
}

func TestConsumeCraftIngredients2x2(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	// Place 4 planks for crafting table
	for i := 1; i <= 4; i++ {
		player.Inventory[i] = Slot{ItemID: 5, Count: 2, Damage: 0}
	}
	consumeCraftIngredients2x2(player)
	for i := 1; i <= 4; i++ {
		if player.Inventory[i].Count != 1 {
			t.Errorf("slot %d: expected count 1, got %d", i, player.Inventory[i].Count)
		}
	}
	// Consume again - should clear all
	consumeCraftIngredients2x2(player)
	for i := 1; i <= 4; i++ {
		if player.Inventory[i].ItemID != -1 {
			t.Errorf("slot %d: expected empty, got %d", i, player.Inventory[i].ItemID)
		}
	}
}

func TestUpdateCraftOutput3x3(t *testing.T) {
	player := &Player{}
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	for i := range player.CraftTableGrid {
		player.CraftTableGrid[i].ItemID = -1
	}
	player.CraftTableOutput.ItemID = -1
	// Set up furnace recipe
	for i := 0; i < 9; i++ {
		player.CraftTableGrid[i] = Slot{ItemID: 4, Count: 1, Damage: 0}
	}
	player.CraftTableGrid[4] = Slot{ItemID: -1} // center empty
	updateCraftOutput3x3(player)
	if player.CraftTableOutput.ItemID != 61 {
		t.Errorf("expected furnace (61) in output, got %d", player.CraftTableOutput.ItemID)
	}
}

func TestConsumeCraftIngredients3x3(t *testing.T) {
	player := &Player{}
	for i := range player.CraftTableGrid {
		player.CraftTableGrid[i] = Slot{ItemID: 4, Count: 2, Damage: 0}
	}
	player.CraftTableGrid[4] = Slot{ItemID: -1} // center empty
	consumeCraftIngredients3x3(player)
	for i := 0; i < 9; i++ {
		if i == 4 {
			continue
		}
		if player.CraftTableGrid[i].Count != 1 {
			t.Errorf("grid[%d]: expected count 1, got %d", i, player.CraftTableGrid[i].Count)
		}
	}
}

func TestMatchRecipeAtOffset(t *testing.T) {
	// Test that a 1x1 recipe can match at different positions in a 3x3 grid
	grid := make([]Slot, 9)
	for i := range grid {
		grid[i].ItemID = -1
	}
	grid[8] = Slot{ItemID: 17, Count: 1, Damage: 0} // bottom-right

	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for log in bottom-right of 3x3")
	}
	if r.ResultID != 5 {
		t.Errorf("expected planks (5), got %d", r.ResultID)
	}
}

func TestFindRecipeWoodenAxeLeft(t *testing.T) {
	// Wooden axe left variant in 3x3 grid at offset (0,0)
	grid := []Slot{
		{ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: 5, Count: 1, Damage: 0}, {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
		{ItemID: -1},                      {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for wooden axe (left)")
	}
	if r.ResultID != 271 {
		t.Errorf("expected wooden axe (271), got %d", r.ResultID)
	}
}

func TestFindRecipeWoodenAxeRight(t *testing.T) {
	// Wooden axe right variant in 3x3 grid at offset (1,0)
	grid := []Slot{
		{ItemID: -1}, {ItemID: 5, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1}, {ItemID: 280, Count: 1, Damage: 0}, {ItemID: 5, Count: 1, Damage: 0},
		{ItemID: -1}, {ItemID: 280, Count: 1, Damage: 0}, {ItemID: -1},
	}
	r := findRecipe(grid, 3)
	if r == nil {
		t.Fatal("expected recipe match for wooden axe (right)")
	}
	if r.ResultID != 271 {
		t.Errorf("expected wooden axe (271), got %d", r.ResultID)
	}
}
