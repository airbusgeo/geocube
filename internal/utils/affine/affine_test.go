package affine

import (
	"fmt"
	"math"
	"testing"

	"github.com/airbusgeo/geocube/internal/utils"
)

const (
	i0 = 600 * 256
	j0 = 300 * 256
	i1 = 601 * 256
	j1 = 307 * 256
)

func test(t *testing.T, prefix string, x0, x1 float64, counter *int) {
	if math.Abs(x0-x1) > 1e-9 {
		t.Errorf("Expected %s %s==%s (diff=%v)", prefix, utils.F64ToS(x0), utils.F64ToS(x1), x0-x1)
		*counter += 1
	}
}

func TestHighPrecision(t *testing.T) {
	// Webmercator origin, zoom=10
	earthRadius := 6378137.0
	ox, oy := -earthRadius*math.Pi, earthRadius*math.Pi
	resolution := 2 * earthRadius * math.Pi / (256 * (1 << 10))

	a := Translation(ox, oy).Multiply(Scale(resolution, -resolution))
	a0 := a.Multiply(Translation(i0, j0))
	n := 0
	for d := 1024.0; d < 16384; d += 256 {
		x0, y0 := a0.Transform(d, d)
		x1, y1 := a.Transform(i0+d, j0+d)
		test(t, fmt.Sprintf("X+(%0.f", d), x0, x1, &n)
		test(t, fmt.Sprintf("Y+(%0.f", d), y0, y1, &n)
	}
	if n != 0 {
		t.Errorf("%d failed", n)
	}
	// Without high precision => Fail
	/*n = 0
	for d := 0.0; d < 16384; d += 256 {
		x0, y0 := a0[1]*d+a0[2]*d+a0[0], a0[4]*d+a0[5]*d+a0[3]
		x1, y1 := a[1]*(i0+d)+a[2]*(j0+d)+a[0], a[4]*(i0+d)+a[5]*(j0+d)+a[3]
		test(t, fmt.Sprintf("withoutHighPrec X+(%0.f", d), x0, x1, &n)
		test(t, fmt.Sprintf("withoutHighPrec Y+(%0.f", d), y0, y1, &n)
	}
	if n != 0 {
		t.Errorf("%d failed", n)
	}*/
}
