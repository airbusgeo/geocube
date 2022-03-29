package pg

import (
	"context"
	"fmt"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/lib/pq"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

// CreateGrid implements GeocubeBackend
func (b Backend) CreateGrid(ctx context.Context, grid *geocube.Grid) (err error) {
	// Create Grid
	_, err = b.pg.ExecContext(ctx, "INSERT INTO geocube.grids (name, description) VALUES ($1, $2)", grid.Name, grid.Description)
	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Grid", "name", grid.Name, "")
	default:
		return pqErrorFormat("CreateGrid.exec: %w", err)
	}

	// Prepare the insertion of cells
	stmt, err := b.pg.PrepareContext(ctx, pq.CopyInSchema("geocube", "cells", "id", "grid", "crs", "srid", "coordinates"))
	if err != nil {
		return pqErrorFormat("CreateGrid.prepare: %w", err)
	}

	// Insertion error management
	defer func() {
		switch pqErrorCode(err) {
		case noError:
		case uniqueViolation:
			if _, value := extractKeyValueFromDetail(err.(*pq.Error)); value != "" {
				err = geocube.NewEntityAlreadyExists("Cell", "id for gridName", value, "")
			} else {
				err = geocube.NewEntityAlreadyExists("", "", "", "Cell: "+err.Error()) // TODO how to properly handle error?
			}
		case checkViolation:
			err = geocube.NewValidationError(err.Error())
		default:
			err = pqErrorFormat("CreateGrid: %w", err)
		}
		err = utils.MergeErrors(true, err, stmt.Close())
	}()

	// Append the cells
	for _, cell := range grid.Cells {
		if _, err = stmt.ExecContext(ctx, cell.ID, grid.Name, cell.CRS, cell.SRID, &cell.Coordinates); err != nil {
			return err
		}
	}

	// Finish
	_, err = stmt.ExecContext(ctx)
	return err
}

// DeleteGrid implements GeocubeBackend
func (b Backend) DeleteGrid(ctx context.Context, gridName string) error {
	if err := b.delete(ctx, "cells", "grid", gridName); err != nil {
		return err
	}
	return b.delete(ctx, "grids", "name", gridName)
}

// ReadGrid implements GeocubeBackend
func (b Backend) ReadGrid(ctx context.Context, name string) (*geocube.Grid, error) {
	grid := geocube.Grid{Name: name}

	// Get Grid
	err := b.pg.QueryRowContext(ctx, "SELECT description FROM geocube.grids WHERE name = $1", name).Scan(&grid.Description)

	switch pqErrorCode(err) {
	case noError:
		return &grid, nil

	case noDataFound:
		return nil, geocube.NewEntityNotFound("Grid", "name", name, "")
	default:
		return nil, pqErrorFormat("ReadGrid: %w", err)
	}
}

// FindGrids implements GeocubeBackend
func (b Backend) FindGrids(ctx context.Context, nameLike string) ([]*geocube.Grid, error) {
	wc := joinClause{}
	if nameLike != "" {
		nameLike, operator := parseLike(nameLike)
		wc.append(" name "+operator+" $%d", nameLike)
	}

	rows, err := b.pg.QueryContext(ctx, "SELECT name, description FROM geocube.grids"+wc.WhereClause(), wc.Parameters...)
	if err != nil {
		return nil, pqErrorFormat("FindGrids: %w", err)
	}
	defer rows.Close()

	grids := []*geocube.Grid{}
	for rows.Next() {
		var g geocube.Grid
		if err := rows.Scan(&g.Name, &g.Description); err != nil {
			return nil, fmt.Errorf("FindGrids: %w", err)
		}
		grids = append(grids, &g)
	}
	return grids, nil
}

// FindCells implements GeocubeBackend
// Returns the cells and the intersection with the AOI
func (b Backend) FindCells(ctx context.Context, gridName string, aoi *geocube.AOI) ([]geocube.Cell, []geom.MultiPolygon, error) {
	// Execute the query
	rows, err := b.pg.QueryContext(ctx,
		`WITH cells(id, crs, srid, coordinates, intersection) as (
			WITH aoi(coordinates) as (values(ST_GeogFromWKB($1)))
			SELECT id, crs, srid, cells.coordinates,
			  CASE WHEN srid=0 THEN ST_Intersection(aoi.coordinates, cells.coordinates)::geometry
			  ELSE ST_Transform(ST_Intersection(ST_Transform(geometry(aoi.coordinates), srid), ST_Transform(geometry(cells.coordinates), srid)), 4326)::geometry END
			FROM geocube.cells, aoi
			WHERE grid=$2 AND ST_Intersects(aoi.coordinates, cells.coordinates)
		)
		SELECT id, crs, srid, coordinates, st_asbinary(ST_Multi(intersection)) FROM cells WHERE NOT ST_IsEmpty(intersection) AND ST_Dimension(intersection) > 1`,
		&aoi.Geometry, gridName)
	if err != nil {
		return nil, nil, pqErrorFormat("FindCells.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	var cells []geocube.Cell
	var intersections []geom.MultiPolygon
	for rows.Next() {
		cell := geocube.Cell{}
		var geom *wkb.MultiPolygon
		if err := rows.Scan(&cell.ID, &cell.CRS, &cell.SRID, &cell.Coordinates, &geom); err != nil {
			return nil, nil, pqErrorFormat("FindCells.scan: %w", err)
		}
		cells = append(cells, cell)
		intersections = append(intersections, *geom.MultiPolygon)
	}

	return cells, intersections, nil
}
