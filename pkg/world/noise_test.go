package world

import (
	"math"
	"testing"
)

func TestPerlinDeterminism(t *testing.T) {
	p1 := NewPerlin(12345)
	p2 := NewPerlin(12345)

	for i := 0; i < 100; i++ {
		x := float64(i) * 0.37
		y := float64(i) * 0.53
		if p1.Noise2D(x, y) != p2.Noise2D(x, y) {
			t.Fatalf("Noise2D not deterministic at (%f, %f)", x, y)
		}
	}
}

func TestPerlinRange(t *testing.T) {
	p := NewPerlin(42)
	for i := 0; i < 10000; i++ {
		x := float64(i)*0.1 - 500
		y := float64(i)*0.07 - 350
		v := p.Noise2D(x, y)
		if v < -1.5 || v > 1.5 {
			t.Errorf("Noise2D(%f, %f) = %f, out of expected range", x, y, v)
		}
	}
}

func TestNoise3DRange(t *testing.T) {
	p := NewPerlin(99)
	for i := 0; i < 5000; i++ {
		x := float64(i)*0.13 - 300
		y := float64(i)*0.07 - 200
		z := float64(i)*0.09 - 100
		v := p.Noise3D(x, y, z)
		if v < -1.5 || v > 1.5 {
			t.Errorf("Noise3D(%f, %f, %f) = %f, out of expected range", x, y, z, v)
		}
	}
}

func TestOctaveNoiseSmoothness(t *testing.T) {
	p := NewPerlin(77)
	// Adjacent samples should not differ wildly
	prev := p.OctaveNoise2D(0, 0, 4, 2.0, 0.5)
	maxDiff := 0.0
	for i := 1; i < 1000; i++ {
		v := p.OctaveNoise2D(float64(i)*0.01, 0, 4, 2.0, 0.5)
		diff := math.Abs(v - prev)
		if diff > maxDiff {
			maxDiff = diff
		}
		prev = v
	}
	if maxDiff > 0.5 {
		t.Errorf("OctaveNoise2D max step difference = %f, expected smooth transitions", maxDiff)
	}
}

func TestDifferentSeeds(t *testing.T) {
	p1 := NewPerlin(1)
	p2 := NewPerlin(2)
	same := 0
	for i := 0; i < 100; i++ {
		x := float64(i) * 0.5
		y := float64(i) * 0.3
		if p1.Noise2D(x, y) == p2.Noise2D(x, y) {
			same++
		}
	}
	if same > 30 {
		t.Errorf("different seeds produced %d/100 identical values", same)
	}
}
