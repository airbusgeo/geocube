package geocube

type persistenceState int

const (
	persistenceStateUNKNOWN persistenceState = iota
	persistenceStateNEW
	persistenceStateCLEAN
	persistenceStateDIRTY
	persistenceStateTODELETE
	persistenceStateDELETED
)

// IsDirty tests whether the entity has to be persisted
func (p persistenceState) IsDirty() bool {
	return p == persistenceStateDIRTY
}

// IsToDelete tests whether the entity has to be deleted
func (p persistenceState) IsToDelete() bool {
	return p == persistenceStateTODELETE
}

// IsNew tests whether the entity has never been persisted
func (p persistenceState) IsNew() bool {
	return p == persistenceStateNEW
}

// IsClean tests whether everything has been persisted
func (p persistenceState) IsClean() bool {
	return p == persistenceStateCLEAN
}

// IsActive returns true if the entity is New, Clean or Dirty
func (p persistenceState) IsActive() bool {
	return p == persistenceStateNEW || p == persistenceStateCLEAN || p == persistenceStateDIRTY
}

// IsDeleted returns true if the entity is deleted
func (p persistenceState) IsDeleted() bool {
	return p == persistenceStateDELETED
}

// Clean sets the status Clean (everything has been persisted)
func (p *persistenceState) Clean() {
	p.mustNotBeDeleted()
	if *p != persistenceStateTODELETE {
		*p = persistenceStateCLEAN
	}
}

// Deleted sets the status Deleted (the entity is not longer reachable)
func (p *persistenceState) Deleted() {
	*p = persistenceStateDELETED
}

// dirty sets the status Dirty (something has to be persisted)
// Can only be set internaly
func (p *persistenceState) dirty() {
	p.mustNotBeDeleted()
	if *p == persistenceStateCLEAN || *p == persistenceStateUNKNOWN {
		*p = persistenceStateDIRTY
	}
}

// toDelete sets the status ToDelete (the entity has to be deleted)
// Can only be set internaly
func (p *persistenceState) toDelete() {
	p.mustNotBeDeleted()
	*p = persistenceStateTODELETE
}

func (p persistenceState) mustNotBeDeleted() {
	if p == persistenceStateDELETED {
		panic("Entity is deleted")
	}
}
