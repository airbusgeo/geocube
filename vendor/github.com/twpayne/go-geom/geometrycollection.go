package geom

// A GeometryCollection is a collection of arbitrary geometries with the same
// SRID.
type GeometryCollection struct {
	geoms []T
	srid  int
}

// NewGeometryCollection returns a new empty GeometryCollection.
func NewGeometryCollection() *GeometryCollection {
	return &GeometryCollection{}
}

// Geom returns the ith geometry in g.
func (g *GeometryCollection) Geom(i int) T {
	return g.geoms[i]
}

// Geoms returns the geometries in g.
func (g *GeometryCollection) Geoms() []T {
	return g.geoms
}

// Layout returns the smallest layout that covers all of the layouts in g's
// geometries.
func (g *GeometryCollection) Layout() Layout {
	maxLayout := NoLayout
	for _, g := range g.geoms {
		switch l := g.Layout(); l {
		case XYZ:
			if maxLayout == XYM {
				maxLayout = XYZM
			} else if l > maxLayout {
				maxLayout = l
			}
		case XYM:
			if maxLayout == XYZ {
				maxLayout = XYZM
			} else if l > maxLayout {
				maxLayout = l
			}
		default:
			if l > maxLayout {
				maxLayout = l
			}
		}
	}
	return maxLayout
}

// NumGeoms returns the number of geometries in g.
func (g *GeometryCollection) NumGeoms() int {
	return len(g.geoms)
}

// Stride returns the stride of g's layout.
func (g *GeometryCollection) Stride() int {
	return g.Layout().Stride()
}

// Bounds returns the bounds of all the geometries in g.
func (g *GeometryCollection) Bounds() *Bounds {
	// FIXME this needs work for mixing layouts, e.g. XYZ and XYM
	b := NewBounds(g.Layout())
	for _, g := range g.geoms {
		b = b.Extend(g)
	}
	return b
}

// Empty returns true if the collection is empty.
func (g *GeometryCollection) Empty() bool {
	return len(g.geoms) == 0
}

// FlatCoords panics.
func (g *GeometryCollection) FlatCoords() []float64 {
	panic("FlatCoords() called on a GeometryCollection")
}

// Ends panics.
func (g *GeometryCollection) Ends() []int {
	panic("Ends() called on a GeometryCollection")
}

// Endss panics.
func (g *GeometryCollection) Endss() [][]int {
	panic("Endss() called on a GeometryCollection")
}

// SRID returns g's SRID.
func (g *GeometryCollection) SRID() int {
	return g.srid
}

// MustPush pushes gs to g. It panics on any error.
func (g *GeometryCollection) MustPush(gs ...T) *GeometryCollection {
	if err := g.Push(gs...); err != nil {
		panic(err)
	}
	return g
}

// Push appends geometries.
func (g *GeometryCollection) Push(gs ...T) error {
	g.geoms = append(g.geoms, gs...)
	return nil
}

// SetSRID sets g's SRID and the SRID of all its elements.
func (g *GeometryCollection) SetSRID(srid int) *GeometryCollection {
	g.srid = srid
	return g
}
