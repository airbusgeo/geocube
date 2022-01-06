package geocube

import (
	"context"
	"fmt"
	"strings"

	gridlib "github.com/airbusgeo/geocube/internal/utils/grid"
	"github.com/twpayne/go-geom"
)

type CustomGridInitializer interface {
	ReadGrid(ctx context.Context, name string) (*Grid, error)
	FindCells(ctx context.Context, gridName string, aoi *AOI) ([]Cell, []geom.MultiPolygon, error)
}

// CustomGrid is a grid where each cell is defined either by Parameters["subgrid"] or by Parameters["resolution"]
type CustomGrid struct {
	Name           string
	activeSubGrids map[string]gridlib.Grid // Only valid after a call to "Covers"
	Flags          []string
	Parameters     map[string]string
	cellsFinder    CustomGridInitializer
}

func newCustomGrid(ctx context.Context, initializer CustomGridInitializer, flags []string, parameters map[string]string) (gridlib.Grid, error) {
	if initializer == nil {
		return nil, fmt.Errorf("newCustomGrid: empty initializer")
	}

	grid := CustomGrid{
		Name:           parameters["grid"],
		Flags:          flags,
		Parameters:     parameters,
		cellsFinder:    initializer,
		activeSubGrids: map[string]gridlib.Grid{},
	}

	// Check that the grid exists
	if _, err := initializer.ReadGrid(ctx, grid.Name); err != nil {
		return nil, fmt.Errorf("newCustomGrid.%w", err)
	}

	if _, ok := parameters["subgrid"]; !ok {
		// If there is no subgrid, check that required parameters are defined
		if _, ok := parameters["resolution"]; !ok {
			return nil, fmt.Errorf("customGrid: need either parameters 'subgrid' or 'resolution'")
		}
		parameters["subgrid"] = "singlecell"
	}
	parameters["grid"] = parameters["subgrid"]

	return &grid, nil
}

// Cell implements gridlib.Grid and returns a Cell with the provided URI (format : CellID/subGridID))
func (cg *CustomGrid) Cell(uri string) (*gridlib.Cell, error) {
	split := strings.SplitN(uri, "/", 2)
	if len(split) < 2 {
		return nil, fmt.Errorf("customGrid.Cell: invalid uri : '" + uri + "' must be CellID/uri")
	}
	grid, ok := cg.activeSubGrids[split[0]]
	if !ok {
		return nil, fmt.Errorf("customGrid.Cell: unknown cell identifier: " + split[0])
	}
	return grid.Cell(split[1])
}

// Covers implements gridlib.Grid
func (cg *CustomGrid) Covers(ctx context.Context, geomAOI *geom.MultiPolygon) (<-chan gridlib.StreamedURI, error) {
	aoi, err := NewAOIFromMultiPolygon(*geomAOI)
	if err != nil {
		return nil, fmt.Errorf("CustomGrid.Covers.%w", err)
	}
	cells, intersections, err := cg.cellsFinder.FindCells(ctx, cg.Name, aoi)
	if err != nil {
		return nil, fmt.Errorf("CustomGrid.Covers.%w", err)
	}

	// For each cell, create a subgrid
	cg.activeSubGrids = map[string]gridlib.Grid{}
	for _, cell := range cells {
		cg.Parameters["crs"] = cell.CRS

		if cg.activeSubGrids[cell.ID], err = gridlib.NewGrid(cg.Flags, cg.Parameters); err != nil {
			return nil, fmt.Errorf("CustomGrid.Covers.%w", err)
		}
	}

	// For each intersection, cover the AOI
	uris := make(chan gridlib.StreamedURI)
	go func() {
		defer close(uris)
		for i, intersection := range intersections {
			grid := cg.activeSubGrids[cells[i].ID]
			cellUris, err := grid.Covers(ctx, &intersection)
			if err != nil {
				uris <- gridlib.StreamedURI{Error: fmt.Errorf("CustomGrid.Covers.%v", err)}
				break
			}
			for cellUri := range cellUris {
				uris <- gridlib.StreamedURI{URI: cells[i].ID + "/" + cellUri.URI}
			}
		}
	}()

	return uris, nil
}
