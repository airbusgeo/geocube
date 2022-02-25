package pg

import (
	"context"
	"time"

	"github.com/airbusgeo/geocube/internal/utils/grid"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/lib/pq"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

//ReadContainers implements GeocubeBackend
func (b Backend) ReadContainers(ctx context.Context, containersURI []string) (containers []*geocube.Container, err error) {
	if len(containersURI) == 0 {
		return nil, nil
	}

	// Get Containers
	rows, err := b.pg.QueryContext(ctx, "SELECT uri, managed, storage_class FROM geocube.containers WHERE uri = ANY($1)", pq.Array(containersURI))
	if err != nil {
		return nil, pqErrorFormat("ReadContainers: %w", err)
	}

	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	idx := preserveOrder(containersURI)
	containers = make([]*geocube.Container, len(idx))
	for rows.Next() {
		c := geocube.Container{}
		if err := rows.Scan(&c.URI, &c.Managed, &c.StorageClass); err != nil {
			return nil, pqErrorFormat("ReadContainers.scan: %w", err)
		}
		containers[idx[c.URI]] = &c
	}

	// Check that all containers have been found
	for uri, i := range idx {
		if containers[i] == nil {
			return nil, geocube.NewEntityNotFound("Container", "uri", uri, "")
		}
	}

	// Fetch datasets
	datasets, err := b.findDatasets(ctx, nil, containersURI, "", nil, nil, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, false)
	if err != nil {
		return nil, err
	}

	for _, d := range datasets {
		c := containers[idx[d.ContainerURI]]
		c.Datasets = append(c.Datasets, d)
	}

	return containers, nil
}

//CreateContainer implements GeocubeBackend
func (b Backend) CreateContainer(ctx context.Context, container *geocube.Container) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.containers (uri, managed, storage_class)"+
			" VALUES ($1, $2, $3)",
		container.URI, container.Managed, container.StorageClass)

	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Container", "uri", container.URI, "")
	default:
		return pqErrorFormat("CreateContainer: %w", err)
	}

	return nil
}

// UpdateContainer implements GeocubeBackend
func (b Backend) UpdateContainer(ctx context.Context, container *geocube.Container) error {
	return nil
}

// DeleteContainer implements GeocubeBackend
func (b Backend) DeleteContainer(ctx context.Context, containerURI string) error {
	return b.delete(ctx, "containers", "uri", containerURI)
}

// DeletePendingContainers implements GeocubeBackend
func (b Backend) DeletePendingContainers(ctx context.Context) (int64, error) {
	// Delete containers
	res, err := b.pg.ExecContext(ctx, "DELETE from geocube.containers c WHERE NOT EXISTS (SELECT NULL FROM geocube.datasets d WHERE c.uri = d.container_uri) AND NOT c.managed")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingContainers: %w", err)
	}

	return res.RowsAffected()
}

//CreateDatasets implements GeocubeBackend
func (b Backend) CreateDatasets(ctx context.Context, datasets []*geocube.Dataset) error {
	if len(datasets) == 0 {
		return nil
	}

	// Prepare the insert
	stmt, err := b.pg.PrepareContext(ctx,
		pq.CopyInSchema("geocube", "datasets", "id", "record_id", "instance_id", "container_uri", "geog", "geom", "shape", "subdir",
			"bands", "status", "dtype", "no_data", "min_value", "max_value", "real_min_value", "real_max_value", "exponent", "overviews"))
	if err != nil {
		return pqErrorFormat("CreateDatasets.prepare: %w", err)
	}
	defer func() {
		if e := stmt.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Append the datasets
	for _, dataset := range datasets {
		if _, err = stmt.ExecContext(ctx, dataset.ID, dataset.RecordID, dataset.InstanceID, dataset.ContainerURI,
			&dataset.GeogShape, &dataset.GeomShape, &dataset.Shape, dataset.ContainerSubDir, pq.Array(dataset.Bands), dataset.Status,
			dataset.DataMapping.DType, dataset.DataMapping.NoData, dataset.DataMapping.Range.Min, dataset.DataMapping.Range.Max,
			dataset.DataMapping.RangeExt.Min, dataset.DataMapping.RangeExt.Max, dataset.DataMapping.Exponent, dataset.Overviews); err != nil {
			return pqErrorFormat("CreateDatasets.append.exec: %w", err)
		}
	}

	// Execute statement
	if _, err = stmt.ExecContext(ctx); err != nil {
		switch pqErrorCode(err) {
		case foreignKeyViolation:
			if key, id := extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
				return geocube.NewEntityNotFound("Dataset", key, id, "")
			}
			return geocube.NewEntityNotFound("", "", "", err.(*pq.Error).Detail)
		default:
			return pqErrorFormat("CreateDatasets.exec: %w", err)
		}
	}

	return nil
}

// DeleteDatasets implements GeocubeBackend
func (b Backend) DeleteDatasets(ctx context.Context, datasetsID []string) error {
	_, err := b.pg.ExecContext(ctx, "DELETE from geocube.datasets WHERE id = ANY($1)", pq.Array(datasetsID))

	if err != nil {
		return pqErrorFormat("DeleteDatasets: %w", err)
	}

	return nil
}

// FindDatasets implements GeocubeBackend
func (b Backend) FindDatasets(ctx context.Context, status geocube.DatasetStatus, containerURI, lockedByJobID string, instancesID, recordsID []string,
	recordTags geocube.Metadata, fromTime, toTime time.Time, geog *proj.GeographicRing, refined *proj.Ring, page, limit int, order bool) (datasets []*geocube.Dataset, err error) {
	var containersURI []string
	if containerURI != "" {
		containersURI = []string{containerURI}
	}
	return b.findDatasets(ctx, []geocube.DatasetStatus{status}, containersURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)
}

// findDatasets is identical to FindDatasets but it can take a list of datasetStatus
func (b Backend) findDatasets(ctx context.Context, status []geocube.DatasetStatus, containersURI []string, lockedByJobID string, instancesID, recordsID []string,
	recordTags geocube.Metadata, fromTime, toTime time.Time, geog *proj.GeographicRing, refined *proj.Ring, page, limit int, order bool) (datasets []*geocube.Dataset, err error) {
	// Create the selectClause
	query := "SELECT d.id, d.record_id, d.instance_id, d.container_uri, d.geog, d.geom, d.shape, d.subdir, d.bands, d.status, " +
		"d.dtype, d.no_data, d.min_value, d.max_value, d.real_min_value, d.real_max_value, d.exponent, d.overviews FROM geocube.datasets d"

	// Append the Join clause (for the lockedByJobID)
	if lockedByJobID != "" {
		query += " JOIN geocube.locked_datasets l ON d.id = l.dataset_id"
	}

	if order || !fromTime.IsZero() || !toTime.IsZero() || len(recordTags) > 0 {
		query += " JOIN geocube.records r ON d.record_id = r.id"
	}

	// Create the Where clause
	wc := whereClause{}

	if len(status) == 1 {
		wc.append("d.status = $%d", status[0])
	} else if len(status) > 1 {
		wc.append("d.status IN ANY($%d)", pq.Array(status))
	}

	if lockedByJobID != "" {
		wc.append("l.job_id = $%d", lockedByJobID)
	}

	if len(containersURI) > 0 {
		wc.append("d.container_uri = ANY($%d)", pq.Array(containersURI))
	}

	if len(instancesID) > 0 {
		wc.append("d.instance_id = ANY($%d)", pq.Array(instancesID))
	}

	if len(recordsID) > 0 {
		wc.append("d.record_id = ANY($%d)", pq.Array(recordsID))
	}

	appendTimeFilters(&wc, fromTime, toTime)

	appendTagsFilters(&wc, recordTags)

	if geog != nil {
		wc.append("ST_Intersects(d.geog,  $%d)", geog)
		if refined != nil {
			wc.append("(CASE WHEN ST_SRID(d.shape) = $%d THEN ST_Relate(d.shape,  $%d, 'T********') ELSE true END)", refined.SRID(), refined)
		}
	}

	// Append the whereClause to the query
	query += wc.WhereClause()

	// Append the order
	if order {
		query += " ORDER BY r.datetime, r.id"
	}

	// Append the limitOffsetClause to the query
	query += limitOffsetClause(page, limit)

	// Execute the query
	rows, err := b.pg.QueryContext(ctx, query, wc.Parameters...)
	if err != nil {
		return nil, pqErrorFormat("FindDatasets.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	for rows.Next() {
		d := geocube.Dataset{}

		err := rows.Scan(&d.ID, &d.RecordID, &d.InstanceID, &d.ContainerURI, &d.GeogShape, &d.GeomShape, &d.Shape, &d.ContainerSubDir, pq.Array(&d.Bands), &d.Status,
			&d.DataMapping.DType, &d.DataMapping.NoData, &d.DataMapping.Range.Min, &d.DataMapping.Range.Max, &d.DataMapping.RangeExt.Min, &d.DataMapping.RangeExt.Max, &d.DataMapping.Exponent, &d.Overviews)
		if err != nil {
			return nil, pqErrorFormat("FindDatasets.scan: %w", err)
		}
		datasets = append(datasets, &d)
	}

	return datasets, nil
}

// ListActiveDatasetsID implements GeocubeBackend
// ListActiveDatasetsID retrieves all the datasets from the list of records representing the given variable
func (b Backend) ListActiveDatasetsID(ctx context.Context, instanceID string, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time) ([]string, error) {

	// Create the selectClause
	query := "SELECT d.id FROM geocube.datasets d"

	// Append the Join clause if necessary
	if !fromTime.IsZero() || !toTime.IsZero() || len(recordTags) > 0 {
		query += " JOIN geocube.records r ON d.record_id = r.id"
	}

	// Create the Where clause
	wc := whereClause{}
	wc.append("d.instance_id = $%d AND status='ACTIVE'", instanceID)

	if len(recordsID) > 0 {
		wc.append(" d.record_id = ANY($%d)", pq.Array(recordsID))
	}

	appendTimeFilters(&wc, fromTime, toTime)

	appendTagsFilters(&wc, recordTags)

	// Execute the query
	rows, err := b.pg.QueryContext(ctx, query+wc.WhereClause(), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("ListActiveDatasetsID: %w", err)
	}

	return scanIdsAndClose(rows)
}

// GetDatasetsGeometryUnion implements GeocubeBackend
func (b Backend) GetDatasetsGeometryUnion(ctx context.Context, lockedByJobID string) (*geom.MultiPolygon, error) {
	var data []byte
	err := b.pg.QueryRowContext(ctx,
		"SELECT ST_AsBinary(ST_MULTI(ST_Union(d.geom))) FROM geocube.datasets d JOIN geocube.locked_datasets l ON l.dataset_id = d.id"+
			" WHERE l.job_id=$1", lockedByJobID).Scan(&data)
	if err != nil {
		return nil, pqErrorFormat("GetDatasetsGeometryUnion.QueryRowContext: %w", err)
	}

	if data == nil {
		return &geom.MultiPolygon{}, nil
	}

	g, err := wkb.Unmarshal(data)
	if err != nil {
		return nil, pqErrorFormat("GetDatasetsGeometryUnion.Unmarshal: %w", err)
	}

	geom, ok := g.(*geom.MultiPolygon)
	if !ok {
		return nil, geocube.NewShouldNeverHappen("Wrong type for union of polygon")
	}
	return geom, nil
}

func (b Backend) ComputeValidShapeFromCell(ctx context.Context, datasetIDS []string, cell *grid.Cell) (*proj.Shape, error) {
	srid := proj.Srid(cell.CRS)

	computeShape := proj.Shape{}
	err := b.pg.QueryRowContext(ctx,
		`WITH intersection(shape) AS (
			SELECT ST_Multi(ST_Intersection(ST_Union(ST_Transform(d.shape,$1::int)),$2)) FROM geocube.datasets d  WHERE d.id = ANY($3)
		)
		SELECT shape from intersection where NOT St_IsEmpty(shape) and st_dimension(shape) > 1`, srid, &cell.Ring, pq.Array(datasetIDS)).Scan(&computeShape)
	switch pqErrorCode(err) {
	case noError:
		return &computeShape, nil
	case noDataFound, noData:
		return nil, geocube.NewEntityNotFound("", "", "", "empty intersection with %v", cell.Ring.Coords())
	default:
		return nil, pqErrorFormat("ComputeValidShapeFromCell: %w", err)
	}
}

// UpdateDatasets implements GeocubeBackend
func (b Backend) UpdateDatasets(ctx context.Context, instanceID string, recordIds []string, dmapping geocube.DataMapping) (map[string]int64, error) {

	// Get impact of the update
	rows, err := b.pg.QueryContext(ctx,
		"SELECT COUNT(*), '(' || dtype || ', ' || no_data || ', ' || min_value || ', ' || max_value || ') -> (' || real_min_value || ', ' || real_max_value || ') e' || exponent"+
			" FROM geocube.datasets WHERE instance_id = $1 and record_is = ANY($2)"+
			" GROUP BY dtype, no_data, min_value, max_value, real_min_value, real_max_value, exponent", instanceID, pq.Array(recordIds))
	if err != nil {
		return nil, pqErrorFormat("UpdateDatasets: %w", err)
	}

	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	results := map[string]int64{}
	for rows.Next() {
		var count int64
		var result string
		if err := rows.Scan(&count, &result); err != nil {
			return nil, pqErrorFormat("ReadContainers.scan: %w", err)
		}
		results[result] = count
	}

	// Update
	_, err = b.pg.ExecContext(ctx,
		"UPDATE geocube.datasets SET no_data = $1, min_value = $2, max_value = $3, real_min_value = $4, real_max_value = $5, exponent = $6"+
			" WHERE instance_id = $7", dmapping.NoData, dmapping.Range.Min, dmapping.Range.Max, dmapping.RangeExt.Min, dmapping.RangeExt.Max, dmapping.Exponent, instanceID)

	switch pqErrorCode(err) {
	case noError:
	default:
		return results, pqErrorFormat("UpdateDatasets: %w", err)
	}

	return results, nil
}
