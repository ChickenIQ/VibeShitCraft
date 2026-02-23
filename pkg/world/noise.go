package world

import "math"

// Perlin implements 2D/3D Perlin noise with a seeded permutation table.
type Perlin struct {
	perm [512]int
}

// NewPerlin creates a Perlin noise generator from a seed.
func NewPerlin(seed int64) *Perlin {
	p := &Perlin{}

	// Initialize permutation table 0-255
	var base [256]int
	for i := range base {
		base[i] = i
	}

	// Fisher-Yates shuffle using a simple LCG seeded from the input seed
	s := seed
	for i := 255; i > 0; i-- {
		s = s*6364136223846793005 + 1442695040888963407
		j := int(uint64(s>>16) % uint64(i+1))
		base[i], base[j] = base[j], base[i]
	}

	// Duplicate for wrapping
	for i := 0; i < 256; i++ {
		p.perm[i] = base[i]
		p.perm[i+256] = base[i]
	}
	return p
}

// fade applies the smoothstep 6t^5 - 15t^4 + 10t^3.
func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

// lerp linearly interpolates between a and b.
func lerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

// grad2D returns the dot product of a pseudo-random gradient and the distance vector.
func grad2D(hash int, x, y float64) float64 {
	switch hash & 3 {
	case 0:
		return x + y
	case 1:
		return -x + y
	case 2:
		return x - y
	default:
		return -x - y
	}
}

// Noise2D computes 2D Perlin noise at (x, y). Returns a value roughly in [-1, 1].
func (p *Perlin) Noise2D(x, y float64) float64 {
	// Unit grid cell containing point
	xi := int(math.Floor(x)) & 255
	yi := int(math.Floor(y)) & 255

	// Relative position in cell
	xf := x - math.Floor(x)
	yf := y - math.Floor(y)

	// Fade curves
	u := fade(xf)
	v := fade(yf)

	// Hash coordinates of the 4 corners
	aa := p.perm[p.perm[xi]+yi]
	ab := p.perm[p.perm[xi]+yi+1]
	ba := p.perm[p.perm[xi+1]+yi]
	bb := p.perm[p.perm[xi+1]+yi+1]

	// Bilinear interpolation of gradients
	x1 := lerp(u, grad2D(aa, xf, yf), grad2D(ba, xf-1, yf))
	x2 := lerp(u, grad2D(ab, xf, yf-1), grad2D(bb, xf-1, yf-1))
	return lerp(v, x1, x2)
}

// OctaveNoise2D computes fractal Brownian motion by summing multiple octaves.
func (p *Perlin) OctaveNoise2D(x, y float64, octaves int, lacunarity, persistence float64) float64 {
	var total float64
	frequency := 1.0
	amplitude := 1.0
	maxAmplitude := 0.0

	for i := 0; i < octaves; i++ {
		total += p.Noise2D(x*frequency, y*frequency) * amplitude
		maxAmplitude += amplitude
		amplitude *= persistence
		frequency *= lacunarity
	}

	return total / maxAmplitude
}

// grad3D returns the dot product of a 3D pseudo-random gradient and the distance vector.
func grad3D(hash int, x, y, z float64) float64 {
	h := hash & 15
	u := x
	if h >= 8 {
		u = y
	}
	v := y
	if h >= 4 {
		if h == 12 || h == 14 {
			v = x
		} else {
			v = z
		}
	}
	if (h & 1) != 0 {
		u = -u
	}
	if (h & 2) != 0 {
		v = -v
	}
	return u + v
}

// Noise3D computes 3D Perlin noise at (x, y, z). Returns a value roughly in [-1, 1].
func (p *Perlin) Noise3D(x, y, z float64) float64 {
	xi := int(math.Floor(x)) & 255
	yi := int(math.Floor(y)) & 255
	zi := int(math.Floor(z)) & 255

	xf := x - math.Floor(x)
	yf := y - math.Floor(y)
	zf := z - math.Floor(z)

	u := fade(xf)
	v := fade(yf)
	w := fade(zf)

	aaa := p.perm[p.perm[p.perm[xi]+yi]+zi]
	aba := p.perm[p.perm[p.perm[xi]+yi+1]+zi]
	aab := p.perm[p.perm[p.perm[xi]+yi]+zi+1]
	abb := p.perm[p.perm[p.perm[xi]+yi+1]+zi+1]
	baa := p.perm[p.perm[p.perm[xi+1]+yi]+zi]
	bba := p.perm[p.perm[p.perm[xi+1]+yi+1]+zi]
	bab := p.perm[p.perm[p.perm[xi+1]+yi]+zi+1]
	bbb := p.perm[p.perm[p.perm[xi+1]+yi+1]+zi+1]

	x1 := lerp(u, grad3D(aaa, xf, yf, zf), grad3D(baa, xf-1, yf, zf))
	x2 := lerp(u, grad3D(aba, xf, yf-1, zf), grad3D(bba, xf-1, yf-1, zf))
	y1 := lerp(v, x1, x2)

	x1 = lerp(u, grad3D(aab, xf, yf, zf-1), grad3D(bab, xf-1, yf, zf-1))
	x2 = lerp(u, grad3D(abb, xf, yf-1, zf-1), grad3D(bbb, xf-1, yf-1, zf-1))
	y2 := lerp(v, x1, x2)

	return lerp(w, y1, y2)
}
