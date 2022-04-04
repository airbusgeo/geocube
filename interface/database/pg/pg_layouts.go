package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
	wc := joinClause{}
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

// FindContainerLayouts implements GeocubeBackend
func (b Backend) FindContainerLayouts(ctx context.Context, instanceId string, geomAOI *geocube.AOI, recordIds []string, recordTags map[string]string, fromTime, toTime time.Time) ([]string, [][]string, error) {
	// Create the selectClause
	query := "SELECT DISTINCT cl.layout_name, cl.container_uri FROM geocube.container_layouts cl JOIN geocube.datasets d ON d.container_uri = cl.container_uri"

	if geomAOI != nil || !fromTime.IsZero() || !toTime.IsZero() || len(recordTags) > 0 {
		query += " JOIN geocube.records r ON d.record_id = r.id"
	}

	if geomAOI != nil {
		query += " JOIN geocube.aoi a ON r.aoi_id = a.id"
	}

	// Create the Where clause
	wc := joinClause{}

	wc.append("d.instance_id = $%d", instanceId)

	if len(recordIds) > 0 {
		wc.append("d.record_id = ANY($%d)", pq.Array(recordIds))
	}

	appendTimeFilters(&wc, fromTime, toTime)

	appendTagsFilters(&wc, recordTags)

	if geomAOI != nil {
		wc.append("ST_Intersects(a.geom, ST_GeomFromWKB($%d,4326))", geomAOI.Geometry)
	}

	// Append the whereClause to the query and the order
	query += wc.WhereClause() + " ORDER BY cl.layout_name"

	// Execute the query
	rows, err := b.pg.QueryContext(ctx, query, wc.Parameters...)
	if err != nil {
		return nil, nil, pqErrorFormat("FindContainerLayouts.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	var layouts []string
	var containers [][]string
	var prevLayout string
	for rows.Next() {
		var layout, container string
		if err := rows.Scan(&layout, &container); err != nil {
			return nil, nil, pqErrorFormat("FindContainerLayouts.scan: %w", err)
		}
		if layout == prevLayout {
			containers[len(containers)-1] = append(containers[len(containers)-1], container)
		} else {
			prevLayout = layout
			layouts = append(layouts, layout)
			containers = append(containers, []string{container})
		}
	}

	return layouts, containers, nil
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
