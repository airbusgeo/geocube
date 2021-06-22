package geocube

import (
	"testing"
)

func TestPalette(t *testing.T) {
	if err := (Palette{Name: "wrong name"}).Validate(); !IsError(err, EntityValidationError) {
		t.Errorf("'wrong name' does not fail")
	}
	if err := (Palette{Name: "no_points"}).Validate(); !IsError(err, EntityValidationError) {
		t.Errorf("'no_points' does not fail")
	}

	p := Palette{
		Name:   "test",
		Points: []colorPoint{{Val: 0, R: 0, G: 0, B: 0, A: 1}, {Val: 0.5, R: 127, G: 0, B: 0, A: 1}, {Val: 1, R: 255, G: 0, B: 0, A: 1}},
	}
	if err := p.Validate(); err != nil {
		t.Error(err)
	}

	p.Points[0].Val = 0.1
	if err := p.Validate(); !IsError(err, EntityValidationError) {
		t.Error(err)
	}
	p.Points[0].Val = 0

	p.Points[2].Val = 0.9
	if err := p.Validate(); !IsError(err, EntityValidationError) {
		t.Error(err)
	}
	p.Points[2].Val = 1

	p.Points = append(p.Points, colorPoint{Val: 0.5})
	if err := p.Validate(); !IsError(err, EntityValidationError) {
		t.Error(err)
	}
	p.Points[3].Val = 1

	p.Points[2].Val = p.Points[1].Val
	if err := p.Validate(); !IsError(err, EntityValidationError) {
		t.Error(err)
	}
}
