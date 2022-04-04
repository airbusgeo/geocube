package pg

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/lib/pq"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

//CreateRecords implements GeocubeBackend
func (b Backend) CreateRecords(ctx context.Context, records []*geocube.Record) (err error) {
	// Prepare the insert
	stmt, err := b.pg.PrepareContext(ctx, pq.CopyInSchema("geocube", "records", "id", "name", "tags", "datetime", "aoi_id"))
	if err != nil {
		return pqErrorFormat("CreateRecords.prepare: %w", err)
	}
	defer func() {
		if e := stmt.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Append the records
	for _, record := range records {
		if _, err = stmt.ExecContext(ctx, record.ID, record.Name, record.Tags, record.Time, record.AOI.ID); err != nil {
			return pqErrorFormat("CreateRecords.append.exec: %w", err)
		}
	}

	// Execute statement
	if _, err = stmt.ExecContext(ctx); err != nil {
		switch pqErrorCode(err) {
		case uniqueViolation:
			if _, value := extractKeyValueFromDetail(err.(*pq.Error)); value != "" {
				return geocube.NewEntityAlreadyExists("Record", "", value, "")
			}
			return geocube.NewEntityAlreadyExists("", "", "", "Record: "+err.Error()) // TODO how to properly handle error?
		case foreignKeyViolation:
			if _, aoiID := extractKeyValueFromDetail(err.(*pq.Error)); aoiID != "" {
				return geocube.NewEntityNotFound("AOI", "id", aoiID, "")
			}
		}
		return pqErrorFormat("CreateRecords.exec: %w", err)
	}

	return nil
}

func appendTimeFilters(wc *joinClause, fromTime, toTime time.Time) {
	if !fromTime.IsZero() {
		wc.append("r.datetime >= $%d", fromTime)
	}
	if !toTime.IsZero() {
		wc.append("r.datetime <= $%d", toTime)
	}
}

func appendTagsFilters(wc *joinClause, tags geocube.Metadata) {
	if len(tags) != 0 {
		for k, v := range tags {
			if v == "" {
				wc.append("r.tags ? $%d", k)
			} else {
				v, operator := parseLike(v)
				wc.append("r.tags -> $%d "+operator+" $%d", k, v)
			}
		}
	}
}

// FindRecords implements GeocubeBackend
func (b Backend) FindRecords(ctx context.Context, namelike string, tags geocube.Metadata, fromTime, toTime time.Time, jobID string, aoi *geocube.AOI, page, limit int, order, loadAOI bool) (records []*geocube.Record, err error) {
	// Create the selectClause
	query := "SELECT DISTINCT r.id, r.name, r.datetime, r.tags, r.aoi_id"
	if loadAOI {
		query += ", st_AsBinary(a.geom)"
	}
	query += " FROM geocube.records r"

	// Append the Join clause (for the jobID)
	if jobID != "" {
		query += " JOIN geocube.datasets d ON r.id = d.record_id JOIN geocube.locked_datasets l ON d.id = l.dataset_id"
	}

	if aoi != nil && aoi.Geometry.NumPolygons() == 0 {
		aoi = nil
	}

	if aoi != nil || loadAOI {
		query += " JOIN geocube.aoi a ON r.aoi_id = a.id"
	}

	// Create the Where clause
	wc := joinClause{}
	if jobID != "" {
		wc.append("l.job_id = $%d", jobID)
	}
	if aoi != nil {
		wc.append("ST_Intersects(a.geom, ST_GeomFromWKB($%d,4326))", aoi.Geometry)
	}
	if namelike != "" {
		namelike, operator := parseLike(namelike)
		wc.append("r.name "+operator+" $%d", namelike)
	}

	appendTimeFilters(&wc, fromTime, toTime)

	appendTagsFilters(&wc, tags)

	// Append the whereClause to the query
	query += wc.WhereClause()

	// Append the order
	if order {
		query += " ORDER BY r.datetime"
	}

	// Append the limitOffsetClause
	query += limitOffsetClause(page, limit)

	// Execute the query
	rows, err := b.pg.QueryContext(ctx, query, wc.Parameters...)
	if err != nil {
		return nil, pqErrorFormat("FindRecords.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	for rows.Next() {
		r := geocube.Record{}
		dst := []interface{}{&r.ID, &r.Name, &r.Time, &r.Tags, &r.AOI.ID}
		if loadAOI {
			dst = append(dst, &r.AOI.Geometry)
		}
		if err := rows.Scan(dst...); err != nil {
			return nil, pqErrorFormat("FindRecords.scan: %w", err)
		}
		records = append(records, &r)
	}

	return records, nil
}

// DeleteRecords implements GeocubeBackend
func (b Backend) DeleteRecords(ctx context.Context, ids []string) (int64, error) {
	res, err := b.pg.ExecContext(ctx, "DELETE FROM geocube.records r WHERE id = ANY($1)", pq.Array(ids))
	switch pqErrorCode(err) {
	case noError:
		return res.RowsAffected()
	case foreignKeyViolation:
		var table, id string
		if _, id = extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
			if tableVal := regexp.MustCompile("table \"(.*)\".").FindStringSubmatch(err.(*pq.Error).Detail); len(tableVal) == 2 {
				table = tableVal[1]
			}
		}
		return 0, geocube.NewDependencyStillExists("records", table, "id", id, "")
	default:
		return 0, pqErrorFormat("DeleteRecords: %w", err)
	}
}

// DeletePendingRecords implements GeocubeBackend
func (b Backend) DeletePendingRecords(ctx context.Context) (int64, error) {
	res, err := b.pg.ExecContext(ctx, "DELETE FROM geocube.records r WHERE NOT EXISTS (SELECT NULL FROM geocube.datasets d WHERE r.id = d.record_id)")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingRecords: %w", err)
	}

	return res.RowsAffected()
}

// ReadRecords implements GeocubeBackend
func (b Backend) ReadRecords(ctx context.Context, ids []string) (records []*geocube.Record, err error) {
	// Execute the query
	rows, err := b.pg.QueryContext(ctx, "SELECT id, name, datetime, tags, aoi_id FROM geocube.records WHERE id = ANY($1)", pq.Array(ids))
	if err != nil {
		return nil, pqErrorFormat("ReadRecords.QueryContext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	idx := preserveOrder(ids)
	records = make([]*geocube.Record, len(idx))
	for rows.Next() {
		r := geocube.Record{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Time, &r.Tags, &r.AOI.ID); err != nil {
			return nil, pqErrorFormat("ReadRecords.scan: %w", err)
		}
		records[idx[r.ID]] = &r
	}

	// Check that all records have been found
	for id, i := range idx {
		if records[i] == nil {
			return nil, geocube.NewEntityNotFound("Record", "id", id, "")
		}
	}

	return records, nil
}

// CreateAOI implements GeocubeBackend
func (b Backend) CreateAOI(ctx context.Context, aoi *geocube.AOI) error {
	hash, err := aoi.HashGeometry()
	if err != nil {
		return fmt.Errorf("CreateAOI.%w", err)
	}

	// Test whether entity already exists
	id, err := b.listAOIID(ctx, hash)
	if err == nil {
		return geocube.NewEntityAlreadyExists("AOI", "id", id, "")
	} else if !geocube.IsError(err, geocube.EntityNotFound) {
		return err
	}

	// Create aoi
	if _, err := b.pg.ExecContext(ctx, "INSERT INTO geocube.aoi (id, hash, geom) VALUES ($1, $2, ST_GeomFromWKB($3,4326))", aoi.ID, hash, &aoi.Geometry); err != nil {
		switch pqErrorCode(err) {
		case uniqueViolation:
			return geocube.NewEntityAlreadyExists("AOI", "id", aoi.ID, "")
		default:
			return pqErrorFormat("CreateAOI.exec: %w", err)
		}
	}

	return nil
}

// ReadAOI implements GeocubeBackend
func (b Backend) ReadAOI(ctx context.Context, aoiID string) (*geocube.AOI, error) {
	var aoi geocube.AOI

	// Get AOI
	err := b.pg.QueryRowContext(ctx,
		"SELECT id, st_AsBinary(geom) FROM geocube.aoi WHERE id = $1", aoiID).Scan(&aoi.ID, &aoi.Geometry)

	switch {
	case err == sql.ErrNoRows:
		// AOI has not been found
		return nil, geocube.NewEntityNotFound("AOI", "id", aoiID, "")

	case err != nil:
		return nil, pqErrorFormat("ReadAOI: %w", err)
	}

	return &aoi, nil
}

func (b Backend) listAOIID(ctx context.Context, hash string) (string, error) {
	var id string

	// Get AOIID
	err := b.pg.QueryRowContext(ctx,
		"SELECT id FROM geocube.aoi WHERE hash = $1", hash).
		Scan(&id)

	switch {
	case err == sql.ErrNoRows:
		// aoi has not been found
		return "", geocube.NewEntityNotFound("AOI", "hash", hash, "")

	case err != nil:
		return "", pqErrorFormat("listAOIID: %w", err)
	}

	return id, nil
}

// GetUnionAOI implements GeocubeBackend
func (b Backend) GetUnionAOI(ctx context.Context, recordsID []string) (*geom.MultiPolygon, error) {
	var data []byte
	err := b.pg.QueryRowContext(ctx,
		"SELECT ST_AsBinary(ST_Union(a.geom)) FROM geocube.aoi a JOIN (SELECT DISTINCT aoi_id FROM geocube.records WHERE id = ANY($1)) r ON r.aoi_id = a.id",
		pq.Array(recordsID)).Scan(&data)

	if err != nil {
		return nil, pqErrorFormat("GetUnionAOI.QueryRowContext: %w", err)
	}

	if data == nil {
		return &geom.MultiPolygon{}, nil
	}

	g, err := wkb.Unmarshal(data)
	if err != nil {
		return nil, pqErrorFormat("GetUnionAOI.Unmarshal: %w", err)
	}

	geom, ok := g.(*geom.MultiPolygon)
	if !ok {
		return nil, geocube.NewShouldNeverHappen("Wrong type for union of multipolygon")
	}

	return geom, nil
}

// DeletePendingAOIs implements GeocubeBackend
func (b Backend) DeletePendingAOIs(ctx context.Context) (int64, error) {
	// Delete aoi
	res, err := b.pg.ExecContext(ctx, "DELETE from geocube.aoi a WHERE NOT EXISTS (SELECT NULL FROM geocube.records r WHERE a.id = r.aoi_id)")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingAOIs: %w", err)
	}

	return res.RowsAffected()
}

// AddRecordsTags add or update existing tags on list of records
func (b Backend) AddRecordsTags(ctx context.Context, ids []string, tags geocube.Metadata) (int64, error) {
	formattedTags := b.formatTags(tags)
	res, err := b.pg.ExecContext(ctx, "UPDATE geocube.records SET tags = tags  || '"+formattedTags+"' :: hstore WHERE id = ANY($1);", pq.Array(ids))
	switch pqErrorCode(err) {
	case noError:
		return res.RowsAffected()
	case foreignKeyViolation:
		var table, id string
		if _, id = extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
			if tableVal := regexp.MustCompile("table \"(.*)\".").FindStringSubmatch(err.(*pq.Error).Detail); len(tableVal) == 2 {
				table = tableVal[1]
			}
		}
		return 0, geocube.NewDependencyStillExists("records", table, "id", id, "")
	default:
		return 0, pqErrorFormat("AddRecordsTags: %w", err)
	}
}

// RemoveRecordsTags remove tags on list of records
func (b Backend) RemoveRecordsTags(ctx context.Context, ids []string, tagsKey []string) (int64, error) {
	var rowAffected int64
	for _, tags := range tagsKey {
		res, err := b.pg.ExecContext(ctx, "UPDATE geocube.records SET tags = delete(tags,'"+tags+"') WHERE id = ANY($1);", pq.Array(ids))
		switch pqErrorCode(err) {
		case noError:
			r, _ := res.RowsAffected()
			rowAffected += r
		case foreignKeyViolation:
			var table, id string
			if _, id = extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
				if tableVal := regexp.MustCompile("table \"(.*)\".").FindStringSubmatch(err.(*pq.Error).Detail); len(tableVal) == 2 {
					table = tableVal[1]
				}
			}
			log.Logger(ctx).Sugar().Debugf(geocube.NewDependencyStillExists("records", table, "id", id, "").Error())
		default:
			log.Logger(ctx).Sugar().Debugf(pqErrorFormat("AddRecordsTags: %w", err).Error())
		}
	}
	return rowAffected, nil
}

func (b Backend) formatTags(tags geocube.Metadata) string {
	var splitTags []string
	for key, value := range tags {
		splitTags = append(splitTags, fmt.Sprintf("\"%s\" => \"%s\"", key, value))

	}
	return strings.Join(splitTags, ",")
}
