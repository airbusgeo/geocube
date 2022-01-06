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

// UnsupportedGrid is raised when the GridName is not supported
type UnsupportedGridErr struct {
	GridName string
}

func (err UnsupportedGridErr) Error() string {
	return fmt.Sprintf("unsupported grid type: " + err.GridName)
}

var ReservedNames = []string{"regular", "singlecell"}

// Cell is a polygon with a resolution on the surface of the Earth defined using either
// a WGS84 polygon or a projected polygon. Which of them is the reference
type Cell struct {
	URI            string
	CRS            *godal.SpatialRef
	PixelToCRS     *affine.Affine // Transform from pixel to crs/geometric coordinates
	SizeX, SizeY   int
	GeographicRing proj.GeographicRing // lon/lat geodetic coordinates
	Ring           proj.Ring           // coordinates in the CRS
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

	return nil, UnsupportedGridErr{grid}
}

func newCell(uri string, crs *godal.SpatialRef, srid int, pixToCRS *affine.Affine, sizeX, sizeY int, p *godal.Transform) *Cell {
	c := Cell{
		URI:        uri,
		CRS:        crs,
		PixelToCRS: pixToCRS,
		SizeX:      sizeX,
		SizeY:      sizeY,
		Ring:       proj.NewRingFromExtent(pixToCRS, sizeX, sizeY, srid),
	}

	// Prepare geometric to geographic transform
	x, y := proj.FlatCoordToXY(c.Ring.FlatCoords())

	// Transform all the coordinate at once
	p.TransformEx(x, y, make([]float64, len(x)), nil)

	// TODO densify

	// Convert to flat_coords
	c.GeographicRing = proj.GeographicRing{Ring: proj.NewRingFlat(4326, proj.XYToFlatCoord(x, y))}

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
		lr := cell.GeographicRing.LinearRing
		polygon := geom.NewPolygonFlat(geom.XY, lr.FlatCoords(), []int{len(lr.FlatCoords())})
		if err = g.Push(polygon); err != nil {
			return "", fmt.Errorf("failed to push polygon: %w", err)
		}

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
