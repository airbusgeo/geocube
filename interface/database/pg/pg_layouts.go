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
		"INSERT INTO geocube.layouts (name, grid_flags, grid_parameters, block_x_size, block_y_size, max_records)"+
			" VALUES ($1, $2, $3, $4, $5, $6)",
		layout.Name, pq.Array(layout.GridFlags), layout.GridParameters, layout.BlockXSize, layout.BlockYSize, layout.MaxRecords)

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
func (b Backend) DeleteLayout(ctx context.Context, name string) error {
	return b.delete(ctx, "layouts", "name", name)
}

// ReadLayout implements GeocubeBackend
func (b Backend) ReadLayout(ctx context.Context, name string) (*geocube.Layout, error) {
	l := geocube.Layout{Name: name}

	// Get Layout
	err := b.pg.QueryRowContext(ctx,
		"SELECT grid_flags, grid_parameters, block_x_size, block_y_size, max_records "+
			"FROM geocube.layouts WHERE name = $1", name).
		Scan(pq.Array(l.GridFlags), &l.GridParameters, &l.BlockXSize, &l.BlockYSize, &l.MaxRecords)

	switch {
	case err == sql.ErrNoRows:
		// Layout has not been found
		return nil, geocube.NewEntityNotFound("Layout", "name", name, "")

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
		"SELECT name, grid_flags, grid_parameters, block_x_size, block_y_size, max_records FROM geocube.layouts"+wc.WhereClause(), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("FindLayouts: %w", err)
	}
	defer rows.Close()

	layouts := []*geocube.Layout{}
	for rows.Next() {
		var l geocube.Layout
		if err := rows.Scan(&l.Name, pq.Array(l.GridFlags), &l.GridParameters, &l.BlockXSize, &l.BlockYSize, &l.MaxRecords); err != nil {
			return nil, fmt.Errorf("FindLayouts: %w", err)
		}
		layouts = append(layouts, &l)
	}
	return layouts, nil
}

// SaveContainerLayout implements GeocubeBackend
func (b Backend) SaveContainerLayout(ctx context.Context, containerURI string, layoutName string) error {
	_, err := b.pg.ExecContext(ctx, "INSERT INTO geocube.container_layouts (container_uri, layout_name) VALUES ($1, $2)", containerURI, layoutName)

	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("ContainerURI", "container_uri", containerURI, "")
	default:
		return pqErrorFormat("saveContainerLayout.exec: %w", err)
	}

	return nil
}

// DeleteContainerLayout implements GeocubeBackend
func (b Backend) DeleteContainerLayout(ctx context.Context, containerURI string) error {
	return b.delete(ctx, "container_layouts", "container_uri", containerURI)
}
