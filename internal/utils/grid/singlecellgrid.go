package grid

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
	"github.com/twpayne/go-geom"
)

func newSingleCellGrid(flags []string, parameters map[string]string) (Grid, error) {
	grid := &SingleCellGrid{}

	var err error
	grid.crs, grid.srid, err = proj.CRSFromUserInput(parameters["crs"])
	if err != nil {
		return nil, invalidError("CRS parameters [%v]: %w", parameters["crs"], err)
	}
	if grid.srid == 0 {
		return nil, invalidError("CRS parameters: unable to retrieve SRID from input")
	}

	resolutions, ok := parameters["resolution"]
	if !ok {
		return nil, fmt.Errorf("SingleCellGrid.newSingleCellGrid failed to found resolution")
	}
	resolution, err := strconv.ParseFloat(resolutions, 64)
	if err != nil {
		return nil, err
	}

	grid.resolution = resolution

	return grid, nil
}

type SingleCellGrid struct {
	crs        *godal.SpatialRef
	srid       int
	resolution float64
}

// Covers
func (cg *SingleCellGrid) Covers(ctx context.Context, geomAOI *geom.MultiPolygon) (<-chan StreamedURI, error) {
	if geomAOI.Stride() == 0 || geomAOI.NumCoords() == 0 {
		return nil, fmt.Errorf("SingleCellGrid.Covers: empty AOI")
	}

	lonLatToCRS, err := proj.CreateLonLatProj(cg.crs, false)
	if err != nil {
		return nil, fmt.Errorf("SingleCellGrid.Covers.%w", err)
	}

	x, y := proj.FlatCoordToXY(geomAOI.FlatCoords())
	if err := lonLatToCRS.TransformEx(x, y, make([]float64, len(x)), nil); err != nil {
		return nil, fmt.Errorf("SingleCellGrid.Covers: failed to transform coord: %w", err)
	}

	// Get the bounds of the AOI in the crs
	crsAOI := geom.NewMultiPolygonFlat(geom.XY, proj.XYToFlatCoord(x, y), geomAOI.Endss())
	b := crsAOI.Bounds()
	if b.IsEmpty() {
		return nil, fmt.Errorf("SingleCellGrid.Covers: error in input geometry: the bounds in the CRS of the grid are empty")
	}

	originX := b.Min(0)
	originY := b.Max(1)

	width := math.Round(math.Abs(b.Min(0)-b.Max(0)) / math.Abs(cg.resolution))
	height := math.Round(math.Abs(b.Min(1)-b.Max(1)) / math.Abs(cg.resolution))

	uris := make(chan StreamedURI, 1)
	uris <- StreamedURI{URI: fmt.Sprintf("%s/%s/%d/%d", utils.F64ToS(originX), utils.F64ToS(originY), int(width), int(height))}
	close(uris)

	return uris, nil

}

// Cell implements Grid and returns the Single Cell given the URI (format : originX/originY/sizeX/sizeY)
func (cg *SingleCellGrid) Cell(uri string) (*Cell, error) {
	var originX, originY float64
	var sizeX, sizeY int
	if n, err := fmt.Sscanf(uri, "%f/%f/%d/%d", &originX, &originY, &sizeX, &sizeY); err != nil || n != 4 {
		return nil, fmt.Errorf("OneCellGrid.Cell format must be originX/originY/sizeX/sizeY: %s", uri)
	}

	pixToCRS := affine.Translation(originX, originY).Multiply(affine.Scale(cg.resolution, -cg.resolution))

	crsToLonLat, err := proj.CreateLonLatProj(cg.crs, true)
	if err != nil {
		return nil, fmt.Errorf("SingleCellGrid.Covers.%w", err)
	}

	return newCell(uri, cg.crs, cg.srid, pixToCRS, sizeX, sizeY, crsToLonLat), nil
}
