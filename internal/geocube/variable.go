package geocube

//go:generate enumer -json -sql -type Resampling -trimprefix Resampling

import (
	"fmt"
	"regexp"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/godal"
	"github.com/google/uuid"
)

// Resampling defines how the raster is resampled when its size has to be changed
type Resampling int32

// Supported values for resampling (gdal resampling methods)
const (
	_ Resampling = iota // For compatibility with protobuf whose 0 is undefined
	ResamplingNEAR
	ResamplingBILINEAR
	ResamplingCUBIC
	ResamplingCUBICSPLINE
	ResamplingLANCZOS
	ResamplingAVERAGE
	ResamplingMODE
	ResamplingMAX
	ResamplingMIN
	ResamplingMED
	ResamplingQ1
	ResamplingQ3
)

// CanInterpolate returns true if the resampling may interpolate values
func (r Resampling) CanInterpolate() bool {
	switch r {
	case ResamplingBILINEAR, ResamplingCUBIC, ResamplingCUBICSPLINE, ResamplingLANCZOS, ResamplingAVERAGE:
		return true
	default:
		return false
	}
}

func (r Resampling) ToGDAL() godal.ResamplingAlg {
	switch r {
	default:
		return godal.Nearest
	case ResamplingBILINEAR:
		return godal.Bilinear
	case ResamplingCUBIC:
		return godal.Cubic
	case ResamplingCUBICSPLINE:
		return godal.CubicSpline
	case ResamplingLANCZOS:
		return godal.Lanczos
	case ResamplingAVERAGE:
		return godal.Average
	case ResamplingMODE:
		return godal.Mode
	case ResamplingMAX:
		return godal.Max
	case ResamplingMIN:
		return godal.Min
	case ResamplingMED:
		return godal.Median
	case ResamplingQ1:
		return godal.Q1
	case ResamplingQ3:
		return godal.Q3
	}
}

// VariableInstance is an instance of the VariableDefinition
type VariableInstance struct {
	persistenceState
	ID       string
	Name     string
	Metadata Metadata
}

// Variable described data
type Variable struct {
	persistenceState
	ID        string
	Name      string
	Instances map[string]*VariableInstance
	Unit      string

	// Description [mutable]
	Description string

	// Storage [immutable]
	Bands   []string
	DFormat DataFormat

	// Views [mutable]
	Palette string

	// Default resampling algorithm [mutable]
	Resampling Resampling

	// Consolidation parameters
	ConsolidationParams ConsolidationParams
}

// NewInstance creates a variable instance and validates it
func NewInstance(name string, metadata map[string]string) (*VariableInstance, error) {
	vi := VariableInstance{
		persistenceState: persistenceStateNEW,
		ID:               uuid.New().String(),
		Name:             name,
		Metadata:         Metadata(metadata),
	}

	if err := vi.validate(); err != nil {
		return nil, err
	}
	return &vi, nil
}

// NewVariableFromProtobuf creates a variable from protobuf and validates it
// Only returns validationError
func NewVariableFromProtobuf(pbv *pb.Variable) (*Variable, error) {
	dformat := NewDataFormatFromProtobuf(pbv.GetDformat())

	if pbv.GetResamplingAlg() == pb.Resampling_UNDEFINED {
		return nil, NewValidationError("Resampling algorithm cannot be undefined")
	}

	v := Variable{
		persistenceState: persistenceStateNEW,
		ID:               uuid.New().String(),
		Name:             pbv.GetName(),
		Unit:             pbv.GetUnit(),
		Description:      pbv.GetDescription(),
		Bands:            pbv.GetBands(),
		DFormat:          *dformat,
		Palette:          pbv.GetPalette(),
		Resampling:       Resampling(pbv.GetResamplingAlg()),
	}

	if err := v.validate(); err != nil {
		return nil, err
	}
	return &v, nil
}

// ToProtobuf converts a variable to a protobuf
func (v *Variable) ToProtobuf() *pb.Variable {
	pbvar := &pb.Variable{
		Id:            v.ID,
		Name:          v.Name,
		Unit:          v.Unit,
		Description:   v.Description,
		Dformat:       v.DFormat.ToProtobuf(),
		Bands:         v.Bands,
		Palette:       v.Palette,
		ResamplingAlg: pb.Resampling(v.Resampling),
		Instances:     make([]*pb.Instance, 0, len(v.Instances)),
	}

	for _, instance := range v.Instances {
		pbvar.Instances = append(pbvar.Instances, &pb.Instance{
			Id:       instance.ID,
			Name:     instance.Name,
			Metadata: instance.Metadata,
		})
	}
	return pbvar
}

// Clean sets the status Clean to the variable and (if "all") all its instances
func (v *Variable) Clean(all bool) {
	if all {
		for _, vi := range v.Instances {
			vi.Clean()
		}
		v.ConsolidationParams.Clean()
	}
	v.persistenceState.Clean()
}

// ToDelete sets the status ToDelete to one instance or to all instances and the variable itselfs
func (v *Variable) ToDelete(instanceID string) {
	if instanceID == "" {
		for _, vi := range v.Instances {
			vi.toDelete()
		}
		v.ConsolidationParams.toDelete()
		v.toDelete()
	} else {
		for _, vi := range v.Instances {
			if vi.ID == instanceID {
				vi.toDelete()
			}
		}
	}
}

// Update updates the variable
func (v *Variable) Update(name, unit, description, palette *string, resampling *Resampling) error {
	if name != nil && v.Name != *name {
		v.Name = *name
		v.dirty()
	}
	if unit != nil && v.Unit != *unit {
		v.Unit = *unit
		v.dirty()
	}
	if description != nil && v.Description != *description {
		v.Description = *description
		v.dirty()
	}
	if palette != nil && v.Palette != *palette {
		v.Palette = *palette
		v.dirty()
	}
	if resampling != nil && v.Resampling != *resampling {
		v.Resampling = *resampling
		v.dirty()
	}

	if v.IsDirty() {
		return v.validate()
	}
	return nil
}

// SetConsolidationParams sets the consolidation parameters
// Only returns ValidationError
func (v *Variable) SetConsolidationParams(params ConsolidationParams) error {
	v.ConsolidationParams = params
	if !v.DFormat.canCastTo(&params.DFormat) || !params.DFormat.canCastTo(&v.DFormat) {
		return NewValidationError("ConsolidationParams: DataFormats are not compatible")
	}
	return nil
}

// CheckInstanceExists checks that the instance belongs to the variable
func (v *Variable) CheckInstanceExists(instanceID string) error {
	if _, ok := v.Instances[instanceID]; !ok {
		return NewEntityNotFound("Instance", "id", instanceID, fmt.Sprintf("Instance %s does not belong to the variable %s (%s)", instanceID, v.Name, v.ID))
	}
	return nil
}

// checkInstanceDoesNotExist checks that no other instance has the same name
func (v *Variable) checkInstanceDoesNotExist(name, exceptInstanceID string) error {
	for _, instance := range v.Instances {
		if instance.ID != exceptInstanceID && instance.Name == name {
			return NewEntityAlreadyExists("Instance", "name", v.Name+":"+name, "")
		}
	}
	return nil
}

// AddInstance adds a new instance if possible
func (v *Variable) AddInstance(vi *VariableInstance) error {
	// Search whether an instance with the same name already exists
	if err := v.checkInstanceDoesNotExist(vi.Name, ""); err != nil {
		return err
	}
	v.Instances[vi.ID] = vi
	return nil
}

// UpdateInstance updates an instance of the variable
func (v *Variable) UpdateInstance(instanceID string, name *string, newMetadata map[string]string, delMetadataKeys []string) error {
	instance := v.Instances[instanceID]

	if name != nil && instance.Name != *name {
		// Search whether an instance with the same name already exists
		if err := v.checkInstanceDoesNotExist(*name, instanceID); err != nil {
			return err
		}
		instance.Name = *name
		instance.dirty()
	}

	// Insert or update new keys
	for key, newVal := range newMetadata {
		if val, ok := instance.Metadata[key]; !ok || val != newVal {
			instance.Metadata[key] = newVal
			instance.dirty()
		}
	}
	// Delete keys
	for _, key := range delMetadataKeys {
		delete(instance.Metadata, key)
		instance.dirty()
	}

	return instance.validate()
}

func (vi *VariableInstance) validate() error {
	if _, err := uuid.Parse(vi.ID); err != nil {
		return NewValidationError("Invalid uuid: " + vi.ID)
	}
	if m, err := regexp.MatchString("^[a-zA-Z0-9-:_]+$", vi.Name); err != nil || !m {
		return NewValidationError("Invalid Name: " + vi.Name)
	}
	return nil
}

func (v *Variable) validate() error {
	if _, err := uuid.Parse(v.ID); err != nil {
		return NewValidationError("Invalid uuid: " + v.ID)
	}

	if !isValidURN(v.Name) {
		return NewValidationError("Incorrect name: " + v.Name)
	}

	if v.Palette != "" && !isValidURN(v.Palette) {
		return NewValidationError("Incorrect palette name: " + v.Palette)
	}

	if v.Palette != "" && len(v.Bands) != 1 {
		return NewValidationError("Cannot define a palette to a multi-bands variable")
	}

	if err := v.DFormat.validate(); err != nil {
		return NewValidationError("Incorrect data format: " + err.Error())
	}

	if len(v.Bands) == 0 {
		return NewValidationError("Bands definition must have at least one band")
	}
	if len(v.Bands) > 1 {
		for _, name := range v.Bands {
			if name == "" {
				return NewValidationError("Band name cannot be empty")
			}
		}
	}
	return nil
}
