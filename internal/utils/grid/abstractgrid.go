package grid

import (
	"context"
	"fmt"
	"strings"

	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

// Cell is a polygon on the surface of the Earth defined using either
// a WGS84 polygon or a projected polygon. Which of them is the reference
type Cell struct {
	URI            string
	CRS            *godal.SpatialRef
	PixelToCRS     *affine.Affine // Transform from pixel to crs/geometric coordinates
	SizeX, SizeY   int
	GeographicRing proj.GeographicShape // lon/lat geodetic coordinates
	Ring           proj.Shape           // coordinates in the CRS
}

type StreamedURI struct {
	URI   string
	Error error
}

type Grid interface {
	// Return the cell defined by the uris
	Cell(uri string) (*Cell, error)

	// Covers streams uris of cells covering the AOI.
	// The uris are unique, but the cells defined by the uris might overlap.
	Covers(ctx context.Context, aoi *geom.MultiPolygon) (<-chan StreamedURI, error)
}

// NewGrid creates a new grid from flag and parameters (proj4 format)
func NewGrid(flags []string, parameters map[string]string) (Grid, error) {
	grid, ok := parameters["grid"]
	if !ok {
		return nil, fmt.Errorf("missing 'grid' in parameters")
	}
	switch strings.ToLower(grid) {
	case "regular":
		return newRegularGrid(flags, parameters)
	case "singlecell":
		return newSingleCellGrid(flags, parameters)
	}

	return nil, fmt.Errorf("unsupported grid type: " + grid)
}

func newCell(uri string, crs *godal.SpatialRef, srid int, pixToCRS *affine.Affine, sizeX, sizeY int, p *godal.Transform) *Cell {
	c := Cell{
		URI:        uri,
		CRS:        crs,
		PixelToCRS: pixToCRS,
		SizeX:      sizeX,
		SizeY:      sizeY,
	}

	x1, y1 := pixToCRS.Transform(0, 0)
	x2, y2 := pixToCRS.Transform(float64(sizeX), float64(sizeY))

	c.Ring = proj.NewShapeFlat(srid, []float64{x1, y1, x1, y2, x2, y2, x2, y1, x1, y1})

	// Prepare geometric to geographic transform
	x, y := proj.FlatCoordToXY(c.Ring.FlatCoords())

	// Transform all the coordinate at once
	p.TransformEx(x, y, make([]float64, len(x)), nil)

	// TODO densify

	// Convert to flat_coords
	c.GeographicRing = proj.GeographicShape{Shape: proj.NewShapeFlat(4326, proj.XYToFlatCoord(x, y))}

	return &c
}

// CellsToJSON converts an array of cells_uri to a geojson
func CellsToJSON(gr Grid, cellsURI []string) (string, error) {
	g := geom.NewMultiPolygon(geom.XY)
	for _, cellURI := range cellsURI {
		cell, err := gr.Cell(cellURI)
		if err != nil {
			return "", fmt.Errorf("unable to retrieve the cell '%s': %w", cellURI, err)
		}
		g.Push(&cell.GeographicRing.Polygon)
	}
	return geomToJSON(g)
}

func geomToJSON(g geom.T) (string, error) {
	// Convert the coordinates to WKB and update feature.geometry
	geomwkb, err := wkb.Marshal(g, wkb.NDR)
	if err != nil {
		return "", err
	}

	srs, err := godal.NewSpatialRefFromEPSG(4326)
	if err != nil {
		return "", err
	}
	geometry, err := godal.NewGeometryFromWKB(geomwkb, srs)
	if err != nil {
		return "", err
	}
	defer geometry.Close()

	return geometry.GeoJSON(godal.SignificantDigits(12))
}
