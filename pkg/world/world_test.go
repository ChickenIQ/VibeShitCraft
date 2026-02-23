package world

import (
	"testing"
)

func TestTreeDetection(t *testing.T) {
	// Use a seed and position where we know (or likely have) a tree.
	// We'll iterate over a small area to find one.
	w := NewWorld(12345)
	
	foundLog := false
	for x := int32(-64); x < 64; x++ {
		for z := int32(-64); z < 64; z++ {
			for y := int32(5); y < 15; y++ {
				block := w.GetBlock(x, y, z)
				blockID := block >> 4
				if blockID == 17 { // Log
					foundLog = true
					t.Logf("Found log at (%d, %d, %d)", x, y, z)
					break
				}
			}
			if foundLog {
				break
			}
		}
		if foundLog {
			break
		}
	}

	if !foundLog {
		t.Errorf("No logs found in area (-64,-64,5)-(64,64,15) for seed 12345.")
	}
}
