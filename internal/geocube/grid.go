package geocube

import (
	"fmt"
	"regexp"
	"strings"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
	gridlib "github.com/airbusgeo/geocube/internal/utils/grid"
	"github.com/airbusgeo/geocube/internal/utils/proj"
)

type Cell struct {
	ID          string
	CRS         string
	SRID        int // Retrieved from CRS (not always possible)
	Coordinates proj.GeographicRing
}

type Grid struct {
	Name        string
	Description string
	Cells       []Cell
}

func sridFromCrs(crs string, crsDict map[string]int) (int, error) {
	if srid, ok := crsDict[crs]; ok {
		return srid, nil
	}
	var err error
	if _, crsDict[crs], err = proj.CRSFromUserInput(crs); err != nil {
		return 0, err
	}
	return crsDict[crs], nil

}

// NewGridFromProtobuf creates a grid from protobuf and validates it
// Only returns validationError
func NewGridFromProtobuf(grid *pb.Grid) (*Grid, error) {
	sridDict := map[string]int{}

	g := Grid{Name: grid.Name, Description: grid.Description, Cells: make([]Cell, 0, len(grid.Cells))}
	for _, pbc := range grid.Cells {
		srid, err := sridFromCrs(pbc.Crs, sridDict)
		if err != nil {
			return nil, fmt.Errorf("NewGridFromProtobuf.%w", err)
		}
		points := pbc.Coordinates.Points
		flatCoords := make([]float64, 2*len(points))
		for i, point := range points {
			flatCoords[2*i] = float64(point.GetLon())
			flatCoords[2*i+1] = float64(point.GetLat())
		}
		g.Cells = append(g.Cells, Cell{
			ID:          pbc.Id,
			CRS:         pbc.Crs,
			SRID:        srid,
			Coordinates: proj.GeographicRing{Ring: proj.NewRingFlat(4326, flatCoords)},
		})
	}

	if err := g.validate(); err != nil {
		return nil, err
	}

	return &g, nil
}

// ToProtobuf converts a consolidationParams to protobuf
func (g *Grid) ToProtobuf() *pb.Grid {
	return &pb.Grid{
		Name:        g.Name,
		Description: g.Description,
	}
}

// validate returns an error if grid has an invalid format
func (g *Grid) validate() error {
	if matched, err := regexp.MatchString("^[a-zA-Z0-9-:_]+$", g.Name); err != nil || !matched {
		return NewValidationError("invalid name: %s", g.Name)
	}
	lowerName := strings.ToLower(g.Name)
	for _, name := range gridlib.ReservedNames {
		if lowerName == name {
			return NewValidationError("reserved name: %s", g.Name)
		}
	}
	cellIds := utils.StringSet{}
	for _, cell := range g.Cells {
		if !isValidURN(cell.ID) {
			return NewValidationError("invalid cell-id: %s", cell.ID)
		}
		if cellIds.Exists(cell.ID) {
			return NewValidationError("Duplicate cell-id:%s", cell.ID)
		}
		cellIds.Push(cell.ID)
	}

	return nil
}
