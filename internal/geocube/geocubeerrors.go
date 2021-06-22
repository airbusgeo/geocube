package geocube

import (
	"errors"
	"fmt"
)

type ErrorCode int

const (
	EntityValidationError ErrorCode = iota
	EntityNotFound
	EntityAlreadyExists
	DependencyStillExists
	UnhandledEvent
	ShouldNeverHappen
)

// Access details
const (
	DetailNotFoundEntity               = 0
	DetailNotFoundKeyID                = 1
	DetailNotFoundID                   = 2
	DetailAlreadyExistsEntity          = 0
	DetailAlreadyExistsKeyID           = 1
	DetailAlreadyExistsID              = 2
	DetailDependencyStillExistsEntity1 = 0
	DetailDependencyStillExistsEntity2 = 1
	DetailDependencyStillExistsKeyID   = 2
	DetailDependencyStillExistsID      = 3
)

type GeocubeError struct {
	code    ErrorCode
	desc    string
	details []string
}

// NewValidationError creates a new validation error
func NewValidationError(desc string, a ...interface{}) error {
	return GeocubeError{code: EntityValidationError, desc: fmt.Sprintf(desc, a...)}
}

// NewEntityNotFound creates a new error stating that an entity has not been found
func NewEntityNotFound(entity, keyID, id, desc string, a ...interface{}) error {
	if desc == "" {
		desc = formatEntityWith(entity, keyID, id)
	}
	return GeocubeError{code: EntityNotFound, desc: fmt.Sprintf(desc, a...), details: []string{entity, keyID, id}}
}

// NewEntityAlreadyExists creates a new error stating that an entity already exists
func NewEntityAlreadyExists(entity, keyID, id, desc string, a ...interface{}) error {
	if desc == "" {
		desc = formatEntityWith(entity, keyID, id)
	}
	return GeocubeError{code: EntityAlreadyExists, desc: fmt.Sprintf(desc, a...), details: []string{entity, keyID, id}}
}

// NewDependencyStillExists creates a new error stating that a dependency between entity still exists and prevents deletion
func NewDependencyStillExists(entity1, entity2, keyID, id, desc string, a ...interface{}) error {
	if desc == "" {
		if entity2 != "" {
			desc = formatEntityWith(entity1+" and "+entity2, keyID, id)
		} else {
			desc = formatEntityWith(entity1, keyID, id)
		}
	}
	return GeocubeError{code: DependencyStillExists, desc: fmt.Sprintf(desc, a...), details: []string{entity1, entity2, keyID, id}}
}

// NewUnhandledEvent creates a new error stating that an event cannot be handled
func NewUnhandledEvent(desc string, a ...interface{}) error {
	return GeocubeError{code: UnhandledEvent, desc: fmt.Sprintf(desc, a...)}
}

// NewShouldNeverHappen creates a new error that should never happen...
func NewShouldNeverHappen(desc string, a ...interface{}) error {
	return GeocubeError{code: ShouldNeverHappen, desc: fmt.Sprintf(desc, a...)}
}

// Error implements error
func (e GeocubeError) Error() string {
	var s string
	switch e.code {
	case EntityValidationError:
		s = "EntityValidationError"
	case EntityNotFound:
		s = "EntityNotFound"
	case EntityAlreadyExists:
		s = "EntityAlreadyExists"
	case DependencyStillExists:
		s = "DependencyStillExists"
	case UnhandledEvent:
		s = "UnhandledEvent"
	case ShouldNeverHappen:
		s = "ShouldNeverHappen"
	}
	return s + ": " + e.desc
}

// Desc returns a description of the error
func (e GeocubeError) Desc() string {
	return e.desc
}

// Code returns the code of the error
func (e GeocubeError) Code() ErrorCode {
	return e.code
}

// Detail returns a detail of the error (see const above)
func (e GeocubeError) Detail(i int) string {
	if i > len(e.details) {
		return ""
	}
	return e.details[i]
}

// IsError tests whether error is a GeocubeError
func IsError(err error, code ErrorCode) bool {
	var gcerr GeocubeError
	return errors.As(err, &gcerr) && gcerr.Code() == code
}

// AsError tests whether error is a GeocubeError and returns it
func AsError(err error, code ErrorCode) (GeocubeError, bool) {
	var gcerr GeocubeError
	return gcerr, errors.As(err, &gcerr) && gcerr.Code() == code
}

func formatEntityWith(entity, keyID, id string) string {
	if entity != "" && id != "" {
		return entity + " with " + keyID + ": " + id
	}
	return ""
}
