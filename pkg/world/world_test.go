package world

import (
	"testing"
)

func TestTreeDetection(t *testing.T) {
	// Use a seed and position where we know (or likely have) a tree.
	// We'll iterate over a small area to find one.
	w := NewWorld(12345)

	foundLog := false
	for x := int32(-100); x < 100; x++ {
		for z := int32(-100); z < 100; z++ {
			surfH := int32(w.Gen.SurfaceHeight(int(x), int(z)))
			// Check a 15-block vertical slice above the surface
			for y := surfH + 1; y < surfH+15; y++ {
				block := w.GetBlock(x, y, z)
				blockID := block >> 4
				if blockID == 17 || blockID == 162 { // Log or Dark Oak Log
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
