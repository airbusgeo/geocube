package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/lib/pq"
)

// CreateLayout implements GeocubeBackend
func (b Backend) CreateLayout(ctx context.Context, layout *geocube.Layout) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.layouts (id, name, grid_flags, grid_parameters, block_x_size, block_y_size, max_records)"+
			" VALUES ($1, $2, $3, $4, $5, $6, $7)",
		layout.ID, layout.Name, pq.Array(layout.GridFlags), layout.GridParameters, layout.BlockXSize, layout.BlockYSize, layout.MaxRecords)

	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Layout", "name", layout.Name, "")
	default:
		return pqErrorFormat("createLayout.exec: %w", err)
	}

	return nil
}

// DeleteLayout implements GeocubeBackend
func (b Backend) DeleteLayout(ctx context.Context, layoutID string) error {
	return b.delete(ctx, "layouts", "id", layoutID)
}

// ReadLayout implements GeocubeBackend
func (b Backend) ReadLayout(ctx context.Context, layoutID string) (*geocube.Layout, error) {
	var l geocube.Layout

	// Get Layout
	err := b.pg.QueryRowContext(ctx,
		"SELECT id, name, grid_flags, grid_parameters, block_x_size, block_y_size, max_records "+
			"FROM geocube.layouts WHERE id = $1", layoutID).
		Scan(&l.ID, &l.Name, pq.Array(l.GridFlags), &l.GridParameters, &l.BlockXSize, &l.BlockYSize, &l.MaxRecords)

	switch {
	case err == sql.ErrNoRows:
		// Layout has not been found
		return nil, geocube.NewEntityNotFound("Layout", "id", layoutID, "")

	case err != nil:
		return nil, pqErrorFormat("ReadLayout: %w", err)
	}

	return &l, nil
}

// FindLayouts implements GeocubeBackend
func (b Backend) FindLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error) {
	wc := whereClause{}
	if nameLike != "" {
		nameLike, operator := parseLike(nameLike)
		wc.append(" name "+operator+" $%d", nameLike)
	}
	rows, err := b.pg.QueryContext(ctx,
		"SELECT id, name, grid_flags, grid_parameters, block_x_size, block_y_size, max_records FROM geocube.layouts"+wc.WhereClause(), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("FindLayouts: %w", err)
	}
	defer rows.Close()

	layouts := []*geocube.Layout{}
	for rows.Next() {
		var l geocube.Layout
		if err := rows.Scan(&l.ID, &l.Name, pq.Array(l.GridFlags), &l.GridParameters, &l.BlockXSize, &l.BlockYSize, &l.MaxRecords); err != nil {
			return nil, fmt.Errorf("FindLayouts: %w", err)
		}
		layouts = append(layouts, &l)
	}
	return layouts, nil
}
