package geocube

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
	gridlib "github.com/airbusgeo/geocube/internal/utils/grid"
	"github.com/twpayne/go-geom"
)

type Layout struct {
	Name string

	// External layout: Grid:Cell (CRS)
	GridFlags      []string
	GridParameters Metadata
	grid           gridlib.Grid

	// Internal layout: Cell, tile
	BlockXSize, BlockYSize int
	MaxRecords             int
}

// NewLayoutFromProtobuf creates a layout from protobuf and validates it
// Only returns validationError
func NewLayoutFromProtobuf(pbl *pb.Layout) (*Layout, error) {
	l := Layout{
		Name:           pbl.GetName(),
		GridFlags:      pbl.GetGridFlags(),
		GridParameters: pbl.GetGridParameters(),
		BlockXSize:     int(pbl.GetBlockXSize()),
		BlockYSize:     int(pbl.GetBlockYSize()),
		MaxRecords:     int(pbl.GetMaxRecords()),
	}

	if err := l.validate(); err != nil {
		return nil, err
	}

	return &l, nil
}

// ToProtobuf converts a layout to protobuf
func (l *Layout) ToProtobuf() *pb.Layout {
	return &pb.Layout{
		Name:           l.Name,
		GridFlags:      l.GridFlags,
		GridParameters: l.GridParameters,
		BlockXSize:     int64(l.BlockXSize),
		BlockYSize:     int64(l.BlockYSize),
		MaxRecords:     int64(l.MaxRecords),
	}
}

type StreamedCell struct {
	*gridlib.Cell
	Error error
}

// Covers returns all the cells of the layout covered by the AOI
func (l *Layout) Covers(ctx context.Context, aoi *geom.MultiPolygon, removeDuplicate bool) (<-chan StreamedCell, error) {
	if l.grid == nil {
		return nil, NewValidationError("covers: grid is not initialized. Call CreateGrid()")
	}
	cellsuri, err := l.grid.Covers(ctx, aoi)
	if err != nil {
		return nil, err
	}

	hashCells := utils.StringSet{}
	cells := make(chan StreamedCell)
	go func() {
		defer close(cells)
		for celluri := range cellsuri {
			cell, err := l.grid.Cell(celluri.URI)
			if err != nil {
				cells <- StreamedCell{Error: fmt.Errorf("Covers.%w", err)}
				continue
			}
			if removeDuplicate {
				hash, err := hashGeometry(&cell.GeographicRing.LinearRing)
				if err != nil {
					cells <- StreamedCell{Error: fmt.Errorf("Covers.%w", err)}
					continue
				}
				if hashCells.Exists(hash) {
					continue
				}
				hashCells.Push(hash)
			}

			select {
			case <-ctx.Done():
				cells <- StreamedCell{Error: fmt.Errorf("Layout.Covers: %w", ctx.Err())}
				return
			case cells <- StreamedCell{Cell: cell}:
			}
		}
	}()
	return cells, nil
}

// validate returns an error if layout has an invalid format
func (l *Layout) validate() error {
	if !isValidURN(l.Name) {
		return NewValidationError("invalid name: " + l.Name)
	}
	if l.BlockXSize <= 0 || l.BlockYSize <= 0 {
		return NewValidationError("blocksize must be positive")
	}
	if l.MaxRecords <= 0 {
		return NewValidationError("maxRecords must be positive")
	}
	return nil
}

// InitGrid if necessary
func (l *Layout) InitGrid(ctx context.Context, initializer CustomGridInitializer) error {
	if l.grid == nil {
		var err error
		l.grid, err = gridlib.NewGrid(l.GridFlags, l.GridParameters)
		verr := gridlib.UnsupportedGridErr{}
		if errors.As(err, &verr) {
			l.grid, err = newCustomGrid(ctx, initializer, l.GridFlags, l.GridParameters)
		}

		if err != nil {
			return fmt.Errorf("InitGrid.%w", err)
		}
	}
	return nil
}
