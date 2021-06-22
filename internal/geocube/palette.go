package geocube

import (
	"database/sql/driver"
	"fmt"
	"image/color"
	"sort"

	pb "github.com/airbusgeo/geocube/internal/pb"
)

type colorPoint struct {
	Val        float32
	R, G, B, A uint8
}

// Palette is a mapping between [0, 1] to RGBA color
type Palette struct {
	persistenceState
	Name   string
	Points []colorPoint
}

// NewPaletteFromPb creates a new palette from pb
// Returns ValidationError
func NewPaletteFromPb(pbp *pb.Palette) (Palette, error) {
	p := Palette{Name: pbp.Name}
	for _, cpt := range pbp.Colors {
		p.Points = append(p.Points, colorPoint{Val: cpt.Value, R: uint8(cpt.R), G: uint8(cpt.G), B: uint8(cpt.B), A: uint8(cpt.A)})
	}
	sort.Slice(p.Points, func(i, j int) bool { return p.Points[i].Val < p.Points[j].Val })

	return p, p.Validate()
}

// Palette256 returns the color.Palette mapping [0, N] to colors
func (p Palette) PaletteN(n int) color.Palette {
	colors := make([]color.Color, n)
	for i, j := 0, 0; i < n; i++ {
		val := float32(i) / float32(n-1)
		for ; p.Points[j+1].Val < val; j++ {
		}
		f := (val - p.Points[j].Val) / (p.Points[j+1].Val - p.Points[j].Val)
		colors[i] = color.RGBA{
			R: uint8(float32(p.Points[j].R)*(1-f) + float32(p.Points[j+1].R)*f),
			G: uint8(float32(p.Points[j].G)*(1-f) + float32(p.Points[j+1].G)*f),
			B: uint8(float32(p.Points[j].B)*(1-f) + float32(p.Points[j+1].B)*f),
			A: uint8(float32(p.Points[j].A)*(1-f) + float32(p.Points[j+1].A)*f),
		}
	}
	return color.Palette(colors)
}

// Validate valids the Palette
func (p Palette) Validate() error {
	if !isValidURN(p.Name) {
		return NewValidationError("Invalid Palette Name: " + p.Name)
	}
	if len(p.Points) < 2 {
		return NewValidationError("Invalid Palette Points: Not enough points (%v)", p.Points)
	}
	if p.Points[0].Val != 0 || p.Points[len(p.Points)-1].Val != 1 {
		return NewValidationError("Invalid Palette Points: first and last values must be 0 and 1 (found %f and %f)", p.Points[0].Val, p.Points[len(p.Points)-1].Val)
	}
	for i := 1; i < len(p.Points); i++ {
		if p.Points[i].Val <= p.Points[i-1].Val {
			return NewValidationError("Invalid Palette Points: values must be strictly increasing (found %f then %f)", p.Points[i-1].Val, p.Points[i].Val)
		}
	}
	return nil
}

// Scan implements the sql.Scanner interface.
func (cpt *colorPoint) Scan(src interface{}) error {
	var s string
	switch src := src.(type) {
	case []byte:
		s = string(src)
	case string:
		s = src
	default:
		return fmt.Errorf("pq: cannot convert %T to colorPoint", src)
	}
	var rgba int
	if _, err := fmt.Sscanf(s, "(%f,%d)", &cpt.Val, &rgba); err != nil {
		return err
	}
	cpt.R, cpt.G, cpt.B, cpt.A = uint8(rgba>>16), uint8(rgba>>8), uint8(rgba), uint8(rgba>>24)
	return nil
}

// Value implements the driver.Valuer interface.
func (cpt colorPoint) Value() (driver.Value, error) {
	v := uint32(cpt.R)<<16 + uint32(cpt.G)<<8 + uint32(cpt.B) + uint32(cpt.A)<<24
	return fmt.Sprintf("(%f,%d)", cpt.Val, v), nil
}
