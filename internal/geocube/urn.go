package geocube

import (
	"database/sql/driver"
	"fmt"
	"regexp"
)

// URN (Unique Ressource Name) for Record, VariableDefinition, Palette... format: Seg1/Seg2/... (allowing special caracters: "-:_")
type URN string

// Scan implements the sql.Scanner interface.
func (urn *URN) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		*urn = URN(string(src))
		return nil
	case string:
		*urn = URN(src)
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T to URN", src)
}

// Value implements the driver.Valuer interface.
func (urn URN) Value() (driver.Value, error) {
	return string(urn), nil
}

func isValidURN(s string) bool {
	matched, err := regexp.MatchString("^[a-zA-Z0-9-:_]+(/[a-zA-Z0-9-:_]+)*$", s)
	return err == nil && matched
}

// at least one character in letters (lower and upper), numbers, `-`, `:` or `_`.
func (urn URN) valid() bool {
	return isValidURN(string(urn))
}

func (urn URN) string() string {
	return string(urn)
}
