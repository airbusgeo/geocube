package grid

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"strconv"

	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

const minCellSize = 1     // Arbitrarly defined for now, but strictly positive
const maxCellSize = 65536 // Arbitrarly defined for now

// RegularGrid in a given CRS with an origin and a spatial resolution
// parameters:
// - "crs" in gdal understandable format
// - "cell_size" or ("cell_x_size", "cell_y_size"): size of the cell
// - "resolution" in crs unit
// - "ox", "oy" origin in crs unit
// - "memory_limit" to prevent crash when covering a large aoi
type RegularGrid struct {
	crs                  *godal.SpatialRef
	srid                 int
	pixToCRS             *affine.Affine
	cellSizeX, cellSizeY int
	lonLatToCRS          *godal.Transform
	crsToLonLat          *godal.Transform
	memoryLimit          int
}

func invalidError(desc string, args ...interface{}) error {
	return fmt.Errorf("Invalid RegularGrid:"+desc, args...)
}

func (rg *RegularGrid) initLonLatToCRS() error {
	return initProjection(&rg.lonLatToCRS, rg.crs, false)
}

func (rg *RegularGrid) initCRSToLonLat() error {
	return initProjection(&rg.crsToLonLat, rg.crs, true)
}

func initProjection(p **godal.Transform, crs *godal.SpatialRef, inverse bool) error {
	if *p == nil {
		var err error
		if *p, err = proj.CreateLonLatProj(crs, inverse); err != nil {
			return err
		}
	}
	return nil
}

func newRegularGrid(flags []string, parameters map[string]string) (Grid, error) {
	grid := RegularGrid{memoryLimit: 9223372036854775807}

	// grid.CRS : Create and initialize the spatial reference from flags & parameters
	var err error
	grid.crs, grid.srid, err = proj.CRSFromUserInput(parameters["crs"])
	if err != nil {
		return nil, invalidError("CRS parameters [\"crs\"=%v]: %w", parameters["crs"], err)
	}
	if grid.srid == 0 {
		return nil, invalidError("CRS parameters: unable to retrieve SRID from input")
	}

	runtime.SetFinalizer(grid.crs, func(crs *godal.SpatialRef) { crs.Close() })

	// grid.CellSize
	{
		if cellSize, ok := parameters["cell_size"]; ok {
			grid.cellSizeX, _ = strconv.Atoi(cellSize)
			grid.cellSizeY = grid.cellSizeX

		} else {
			cellSizeX, okX := parameters["cell_x_size"]
			cellSizeY, okY := parameters["cell_y_size"]
			if okX && okY {
				grid.cellSizeX, _ = strconv.Atoi(cellSizeX)
				grid.cellSizeY, _ = strconv.Atoi(cellSizeY)
			}
		}
		if grid.cellSizeX < minCellSize || grid.cellSizeX > maxCellSize || grid.cellSizeY < minCellSize || grid.cellSizeY > maxCellSize {
			return nil, invalidError("CellSize parameters: must contain a valid 'cell_size', 'cell_x_size' or 'cell_y_size' in [%d, %d]", minCellSize, maxCellSize)
		}
	}

	// grid.Transform
	{
		// Resolution
		resolutions := parameters["resolution"]
		resolution, _ := strconv.ParseFloat(resolutions, 64)
		if resolution <= 0 {
			return nil, invalidError("Resolution parameters: must contain a valid 'resolution'")
		}

		//OriginX & OriginY
		var originX, originY float64
		var err error
		if ox, ok := parameters["ox"]; ok {
			originX, err = strconv.ParseFloat(ox, 64)
			if err != nil {
				return nil, invalidError("Ox invalid parameter: " + ox)
			}
		}
		if oy, ok := parameters["oy"]; ok {
			originY, err = strconv.ParseFloat(oy, 64)
			if err != nil {
				return nil, invalidError("Oy invalid parameter: " + oy)
			}
		}

		// Scale and translate
		grid.pixToCRS = affine.Translation(originX, originY).Multiply(affine.Scale(resolution, -resolution))
	}

	// Memory limit
	if mem, ok := parameters["memory_limit"]; ok {
		if grid.memoryLimit, err = strconv.Atoi(mem); err != nil {
			return nil, invalidError("Memory limit[%s]: %w", mem, err)
		}
	}

	return &grid, nil
}

// Cell implements Grid and returns a Cell in the regular grid with the provided URI (format : i/j))
func (rg *RegularGrid) Cell(uri string) (*Cell, error) {
	var i, j int
	if n, err := fmt.Sscanf(uri, "%d/%d", &i, &j); err != nil || n != 2 {
		return nil, invalidError("Cell format must be 'i/j' as integers")
	}

	cellToCRS := rg.pixToCRS.Multiply(affine.Translation(float64(i*rg.cellSizeX), float64(j*rg.cellSizeY)))

	if err := rg.initCRSToLonLat(); err != nil {
		return nil, fmt.Errorf("unable to initialize projection: %w", err)
	}
	return newCell(uri, rg.crs, rg.srid, cellToCRS, rg.cellSizeX, rg.cellSizeY, rg.crsToLonLat), nil
}

// Create a Dataset containing a geometry
// The caller is responsible to close the output dataset
func createGeometryFromWKB(g *geom.MultiPolygon, crs *godal.SpatialRef) (*godal.Geometry, error) {
	// Convert the input geometry to WKB and update feature.geometry
	geomwkb, err := wkb.Marshal(g, wkb.NDR)
	if err != nil {
		return nil, err
	}

	geometry, err := godal.NewGeometryFromWKB(geomwkb, crs)
	if err != nil {
		return nil, err
	}

	geometry.SetSpatialRef(crs)

	return geometry, nil
}

// Covers implements Grid
func (rg *RegularGrid) Covers(ctx context.Context, geomAOI *geom.MultiPolygon) (<-chan StreamedURI, error) {
	if geomAOI.Stride() == 0 || geomAOI.NumCoords() == 0 {
		return nil, fmt.Errorf("Covers: empty AOI")
	}
	// Transform coordinates from (lon, lat) to CRS coordinates
	if err := rg.initLonLatToCRS(); err != nil {
		return nil, fmt.Errorf("Covers: Unable to initialize projection: %w", err)
	}
	x, y := proj.FlatCoordToXY(geomAOI.FlatCoords())
	if err := rg.lonLatToCRS.TransformEx(x, y, make([]float64, len(x)), nil); err != nil {
		return nil, err
	}
	crsAOI := geom.NewMultiPolygonFlat(geom.XY, proj.XYToFlatCoord(x, y), geomAOI.Endss())
	// Create the dataset which contains the coordinates in the given crs
	geometryIn, err := createGeometryFromWKB(crsAOI, rg.crs)
	if err != nil {
		return nil, fmt.Errorf("Covers: Unable to create a geometry %w", err)
	}

	// Get the bounds of the AOI
	b := crsAOI.Bounds()
	if b.IsEmpty() {
		return nil, errors.New("Covers: Error in input geometry: the bounds are empty")
	}

	// Create the transformation which maps crs coordinates (x, y) to cell coordinates (i, j)
	cellToCRS := rg.pixToCRS.Multiply(affine.Scale(float64(rg.cellSizeX), float64(rg.cellSizeY)))
	crsToCell := cellToCRS.Inverse()

	// Get the bounds in cell coordinates
	i0f, j0f := crsToCell.Transform(b.Min(0), b.Max(1))
	i1f, j1f := crsToCell.Transform(b.Max(0), b.Min(1))
	i0f, j0f = math.Floor(i0f-1), math.Floor(j0f-1)
	i1f, j1f = math.Ceil(i1f+1), math.Ceil(j1f+1)

	// Retrieve the equivalent coordinates in the given crs
	ulX, ulY := cellToCRS.Transform(i0f, j0f)
	lrX, lrY := cellToCRS.Transform(i1f, j1f)

	width := int(math.Abs(ulX-lrX) / math.Abs(cellToCRS.Rx()))
	height := int(math.Abs(ulY-lrY) / math.Abs(cellToCRS.Ry()))
	if width*height*2 > rg.memoryLimit {
		return nil, fmt.Errorf("Covers: Not enough memory (needed:%d, provided:%d)", width*height*2, rg.memoryLimit)
	}
	ds, err := godal.Create(godal.Memory, "", 1, godal.Byte, width, height)
	if err != nil {
		return nil, err
	}

	defer ds.Close()

	if err := ds.SetSpatialRef(rg.crs); err != nil {
		return nil, err
	}

	if err := ds.SetGeoTransform([6]float64{ulX, cellToCRS.Rx(), 0, ulY, 0, cellToCRS.Ry()}); err != nil {
		return nil, err
	}

	if err := ds.RasterizeGeometry(geometryIn, godal.Bands(0), godal.Values(255), godal.AllTouched()); err != nil {
		return nil, fmt.Errorf("Covers: Fail to rasterize: %w", err)
	}

	// Get the image where each pixel is a cell and each non-zero pixel is covered by the aoi.
	img, err := rg.getGrayImage(ds)
	if err != nil {
		return nil, fmt.Errorf("getGrayImage: %w", err)
	}

	// Get the coordinates of non zero pixels
	uris := make(chan StreamedURI)

	go func() {
		defer close(uris)
		s := img.Rect.Size()
		i0, j0 := int(i0f), int(j0f)
		for j := 0; j < s.Y; j++ {
			for i := 0; i < s.X; i++ {
				if img.Pix[j*img.Stride+i] != 0 {
					uris <- StreamedURI{URI: fmt.Sprintf("%d/%d", i+i0, j+j0)}
				}
			}
			select {
			case <-ctx.Done():
				uris <- StreamedURI{Error: fmt.Errorf("RegularGrid.Covers: %w", ctx.Err())}
				return
			default:
			}
		}
	}()

	return uris, nil
}

func (rg *RegularGrid) getGrayImage(d *godal.Dataset) (*image.Gray, error) {
	xSize := d.Structure().SizeX
	ySize := d.Structure().SizeY

	band := d.Bands()[0]

	img := image.NewGray(image.Rect(0, 0, xSize, ySize))
	if err := band.Read(0, 0, img.Pix, xSize, ySize); err != nil {
		return nil, err
	}

	return img, nil
}
