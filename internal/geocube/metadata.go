package geocube

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/lib/pq/hstore"
)

// Metadata adds information to the entity
type Metadata map[string]string

// Scan implements the sql.Scanner interface.
func (m *Metadata) Scan(src interface{}) error {
	switch src := src.(type) {
	case hstore.Hstore:
		*m = Metadata{}
		for key, value := range src.Map {
			(*m)[key] = value.String
		}
		return nil
	case []uint8:
		h := hstore.Hstore{}
		if err := h.Scan(src); err != nil {
			return err
		}
		return m.Scan(h)
	case nil:
		*m = Metadata{}
		return nil
	}

	return fmt.Errorf("cannot convert %T to Metadata", src)
}

// Value implements the driver.Valuer interface.
func (m Metadata) Value() (driver.Value, error) {
	if m == nil {
		return "", nil
	}

	data := hstore.Hstore{Map: map[string]sql.NullString{}}
	for key, value := range m {
		data.Map[key] = sql.NullString{String: value, Valid: true}
	}
	v, err := data.Value()
	if v == nil {
		return nil, err
	}
	return string(v.([]byte)), nil
}
