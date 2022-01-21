package pg

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/lib/pq"
)

var sqlSelectVariable = "SELECT v.id, v.name, v.unit, v.description, v.bands," +
	" v.dtype, v.no_data, v.min_value, v.max_value, v.palette, v.resampling_alg"
var sqlVariableInstance = ", vi.id, vi.name, vi.metadata"

func scanSelect(v *geocube.Variable, vi *geocube.VariableInstance) []interface{} {
	res := []interface{}{&v.ID, &v.Name, &v.Unit, &v.Description, pq.Array(&v.Bands),
		&v.DFormat.DType, &v.DFormat.NoData, &v.DFormat.Range.Min, &v.DFormat.Range.Max,
		&pqPalette{&v.Palette}, &v.Resampling}
	if vi != nil {
		res = append(res, &vi.ID, &vi.Name, &vi.Metadata)
	}
	return res
}

type pqPalette struct{ *string }

func (s *pqPalette) Scan(value interface{}) error {
	if value == nil {
		*s.string = ""
		return nil
	}
	strVal, ok := value.(string)
	if !ok {
		return fmt.Errorf("column is not a string")
	}
	*s.string = strVal
	return nil
}

func (s pqPalette) Value() (driver.Value, error) {
	if len(*s.string) == 0 {
		return nil, nil
	}
	return string(*s.string), nil
}

// ReadVariable implements GeocubeBackend
func (b Backend) ReadVariable(ctx context.Context, variableID string) (*geocube.Variable, error) {
	return b.readVariable(ctx, "id", variableID)
}

// ReadVariableFromName implements GeocubeBackend
func (b Backend) ReadVariableFromName(ctx context.Context, variableName string) (*geocube.Variable, error) {
	return b.readVariable(ctx, "name", variableName)
}

func (b Backend) readVariable(ctx context.Context, field, id string) (*geocube.Variable, error) {
	var v geocube.Variable

	err := b.pg.QueryRowContext(ctx, sqlSelectVariable+" FROM geocube.variable_definitions v WHERE "+field+" = $1", id).Scan(scanSelect(&v, nil)...)

	switch {
	case err == sql.ErrNoRows:
		return nil, geocube.NewEntityNotFound("Variable", field, id, "")
	case err != nil:
		return nil, pqErrorFormat("readvariable: %w", err)
	}

	// Fetch instances
	if v.Instances, err = b.readInstancesFromVariableID(ctx, v.ID); err != nil {
		return nil, err
	}

	return &v, nil
}

// ReadVariableFromInstanceID implements GeocubeBackend
func (b Backend) ReadVariableFromInstanceID(ctx context.Context, instanceID string) (*geocube.Variable, error) {
	var v geocube.Variable
	var vi geocube.VariableInstance

	err := b.pg.QueryRowContext(ctx, sqlSelectVariable+sqlVariableInstance+
		" FROM geocube.variable_definitions v"+
		" JOIN geocube.variable_instances vi ON v.id = vi.definition_id"+
		" WHERE vi.id = $1", instanceID).Scan(scanSelect(&v, &vi)...)

	switch {
	case err == sql.ErrNoRows:
		return nil, geocube.NewEntityNotFound("Variable Instance", "id", instanceID, "")
	case err != nil:
		return nil, pqErrorFormat("ReadVariableFromInstanceID: %w", err)
	}

	v.Instances = map[string]*geocube.VariableInstance{vi.ID: &vi}
	return &v, nil
}

func (b Backend) readInstancesFromVariableID(ctx context.Context, variableID string) (_ map[string]*geocube.VariableInstance, err error) {
	instances := map[string]*geocube.VariableInstance{}

	rows, err := b.pg.QueryContext(ctx, "SELECT id,name,metadata FROM geocube.variable_instances "+
		"WHERE definition_id = $1", variableID)
	if err != nil {
		return nil, pqErrorFormat("readInstancesFromVariableID.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	for rows.Next() {
		vi := geocube.VariableInstance{}

		err := rows.Scan(&vi.ID, &vi.Name, &vi.Metadata)
		if err != nil {
			return nil, pqErrorFormat("readInstancesFromVariableID.scan: %w", err)
		}
		instances[vi.ID] = &vi
	}

	return instances, nil
}

// FindVariables implements GeocubeBackend
func (b Backend) FindVariables(ctx context.Context, namelike string, page, limit int) ([]*geocube.Variable, error) {
	// Create the selectClause
	query := sqlSelectVariable + " FROM geocube.variable_definitions v"

	// Create the Where clause
	wc := whereClause{}
	if namelike != "" {
		namelike, operator := parseLike(namelike)
		wc.append("v.name "+operator+" $%d", namelike)
	}

	// Append the whereClause to the query
	query += wc.WhereClause()

	// Append the limitOffsetClause to the query
	query += limitOffsetClause(page, limit)

	// Execute the query
	rows, err := b.pg.QueryContext(ctx, query, wc.Parameters...)
	if err != nil {
		return nil, pqErrorFormat("FindVariables.querycontext: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Parse rows
	var variables []*geocube.Variable
	for rows.Next() {
		var v geocube.Variable
		if err := rows.Scan(scanSelect(&v, nil)...); err != nil {
			return nil, pqErrorFormat("FindVariables.scan: %w", err)
		}
		if v.Instances, err = b.readInstancesFromVariableID(ctx, v.ID); err != nil {
			return nil, err
		}

		variables = append(variables, &v)
	}

	return variables, nil
}

// CreateVariable implements GeocubeBackend
func (b Backend) CreateVariable(ctx context.Context, variable *geocube.Variable) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.variable_definitions "+
			"(id, name, unit, description, bands, dtype, no_data, min_value, max_value, palette, resampling_alg)"+
			" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
		variable.ID, variable.Name, variable.Unit, variable.Description, pq.Array(variable.Bands),
		variable.DFormat.DType, variable.DFormat.NoData, variable.DFormat.Range.Min, variable.DFormat.Range.Max,
		pqPalette{&variable.Palette}, variable.Resampling)

	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Variable", "name", variable.Name, "")
	default:
		return pqErrorFormat("CreateVariable: %w", err)
	}

	return nil
}

// UpdateVariable implements GeocubeBackend
func (b Backend) UpdateVariable(ctx context.Context, variable *geocube.Variable) error {
	res, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.variable_definitions SET name=$1, unit=$2, description=$3, palette=$4, resampling_alg=$5 WHERE id=$6",
		variable.Name, variable.Unit, variable.Description, pqPalette{&variable.Palette}, variable.Resampling, variable.ID)

	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityNotFound("Variable", "id", variable.ID, "")
		}
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Variable", "name", variable.Name, "")
	default:
		return pqErrorFormat("UpdateVariable: %w", err)
	}

	return nil
}

// DeleteVariable implements GeocubeBackend
func (b Backend) DeleteVariable(ctx context.Context, variableID string) error {
	return b.delete(ctx, "variable_definitions", "id", variableID)
}

// DeletePendingVariables implements GeocubeBackend
func (b Backend) DeletePendingVariables(ctx context.Context) (int64, error) {
	// Delete aoi
	res, err := b.pg.ExecContext(ctx, "DELETE from geocube.variable_definitions d WHERE NOT EXISTS (SELECT NULL FROM geocube.variable_instances i WHERE d.id = i.definition_id)")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingVariables: %w", err)
	}

	return res.RowsAffected()
}

// CreateInstance implements GeocubeBackend
func (b Backend) CreateInstance(ctx context.Context, variableID string, instance *geocube.VariableInstance) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.variable_instances (id, name, definition_id, metadata)"+
			" VALUES ($1, $2, $3, $4)", instance.ID, instance.Name, variableID, instance.Metadata)

	switch pqErrorCode(err) {
	case noError:
		return nil
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Instance", "name", instance.Name, "")
	default:
		return pqErrorFormat("CreateInstance: %w", err)
	}
}

// UpdateInstance updates the name and the metadata of the instance in the database
func (b Backend) UpdateInstance(ctx context.Context, instance *geocube.VariableInstance) error {
	res, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.variable_instances SET name=$1, metadata=$2 WHERE id=$3",
		instance.Name, instance.Metadata, instance.ID)

	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityNotFound("Instance", "id", instance.ID, "")
		}
		return nil
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Instance", "name", instance.Name, "")
	default:
		return pqErrorFormat("UpdateInstance: %w", err)
	}
}

// DeleteInstance implements GeocubeBackend
func (b Backend) DeleteInstance(ctx context.Context, instanceID string) error {
	return b.delete(ctx, "variable_instances", "id", instanceID)
}

// DeletePendingInstances implements GeocubeBackend
func (b Backend) DeletePendingInstances(ctx context.Context) (int64, error) {
	// Delete instances
	res, err := b.pg.ExecContext(ctx, "DELETE from geocube.variable_instances i WHERE NOT EXISTS (SELECT NULL FROM geocube.datasets d WHERE i.id = d.instance_id)")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingInstances: %w", err)
	}

	return res.RowsAffected()
}

// CreatePalette implements GeocubeBackend
func (b Backend) CreatePalette(ctx context.Context, palette *geocube.Palette) error {
	res, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.palette (name, points) VALUES ($1, $2::geocube.color_point[]) ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name WHERE palette.points = EXCLUDED.points", palette.Name, pq.Array(palette.Points))
	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityAlreadyExists("Palette", "name", string(palette.Name), "")
		}
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Palette", "name", string(palette.Name), "")
	default:
		return pqErrorFormat("CreatePalette: %w", err)
	}

	return nil
}

// ReadPalette implements GeocubeBackend
func (b Backend) ReadPalette(ctx context.Context, name string) (*geocube.Palette, error) {
	p := geocube.Palette{Name: name}

	err := b.pg.QueryRowContext(ctx, "SELECT points from geocube.palette WHERE name=$1", name).Scan(pq.Array(&p.Points))

	switch {
	case err == sql.ErrNoRows:
		return nil, geocube.NewEntityNotFound("Palette", "name", name, "")
	case err != nil:
		return nil, pqErrorFormat("ReadPalette: %w", err)
	}
	return &p, nil
}

// UpdatePalette implements GeocubeBackend
func (b Backend) UpdatePalette(ctx context.Context, palette *geocube.Palette) error {
	res, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.palette SET points=$1::geocube.color_point[] WHERE name=$2", pq.Array(palette.Points), palette.Name)

	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityNotFound("Palette", "name", string(palette.Name), "")
		}
		return nil
	default:
		return pqErrorFormat("UpdatePalette: %w", err)
	}
}

// DeletePalette implements GeocubeBackend
func (b Backend) DeletePalette(ctx context.Context, name string) error {
	return b.delete(ctx, "palette", "name", name)
}
