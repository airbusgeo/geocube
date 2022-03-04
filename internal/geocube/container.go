package geocube

//go:generate enumer -json -sql -type StorageClass -trimprefix StorageClass

import (
	"strings"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
)

type StorageClass int

const (
	StorageClassSTANDARD StorageClass = iota
	StorageClassINFREQUENT
	StorageClassARCHIVE
	StorageClassDEEPARCHIVE
	StorageClassUNDEFINED
)

type Container struct {
	persistenceState
	URI          string
	Managed      bool
	StorageClass StorageClass
	Datasets     []*Dataset
}

// NewContainerFromProtobuf creates a new container from protobuf
// Only returns validationError
func NewContainerFromProtobuf(pbc *pb.Container) (*Container, error) {
	c := Container{
		persistenceState: persistenceStateNEW,
		URI:              pbc.GetUri(),
		Managed:          pbc.GetManaged(),
		StorageClass:     StorageClassUNDEFINED}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// NewContainerFromConsolidation creates a new container from the output of a consolidation task
// Only returns ValidationError
func NewContainerFromConsolidation(oc *ConsolidationContainer) (*Container, error) {
	var err error
	c := Container{
		persistenceState: persistenceStateNEW,
		URI:              oc.URI,
		Managed:          true,
		StorageClass:     oc.StorageClass,
	}
	if err = c.validate(); err != nil {
		return nil, NewValidationError(err.Error())
	}
	return &c, nil
}

func (c Container) validate() error {
	//TODO validate URI
	return nil
}

// Clean set the status Clean to the container and (if "all") all its datasets
func (c *Container) Clean(all bool) {
	if all {
		for _, d := range c.Datasets {
			d.Clean()
		}
	}
	c.persistenceState.Clean()
}

// Delete set the status Delete to the container iif it is not managed and all the datasets are inactive
func (c *Container) Delete() error {
	if c.Managed {
		return NewDependencyStillExists("Container", "", "uri", c.URI, "Unable to delete the managed container "+c.URI)
	}
	if c.numActiveDatasets() != 0 {
		return NewDependencyStillExists("Container", "Datasets", "uri", c.URI, "Unable to delete the container "+c.URI+": one of the datasets still exists.")
	}
	c.persistenceState.toDelete()
	return nil
}

// SetStorageClass of the container. It is only possible when the container is new.
func (c *Container) SetStorageClass(storageClass StorageClass) error {
	if !c.IsNew() {
		return NewValidationError("Set storage class but container is not new")
	}
	c.StorageClass = storageClass
	return nil
}

func bandsIntersect(a, b []int64) bool {
	for _, e1 := range a {
		for _, e2 := range b {
			if e1 == e2 {
				return true
			}
		}
	}
	return false
}

// AddDataset to a container
// If the dataset already exists and is exactly the same, no errors are returned
// Only returns EntityAlreadyExists
func (c *Container) AddDataset(d *Dataset) error {
	if !d.IsNew() {
		return NewValidationError("AddDataset: dataset is not new")
	}

	for _, dataset := range c.Datasets {
		if dataset.ContainerSubDir == d.ContainerSubDir && bandsIntersect(dataset.Bands, d.Bands) {
			// Check whether it's the same dataset
			if dataset.identicalTo(d) {
				return nil
			}
			bs := utils.JoinInt64(d.Bands, "-")
			return NewEntityAlreadyExists("dataset", "subdir/bands", d.ContainerSubDir+"/"+bs,
				"A different dataset (record, instance or dformat) already refers to the container "+c.URI+", the subdir '"+d.ContainerSubDir+"' and one of the bands among: "+bs)
		}
	}

	c.Datasets = append(c.Datasets, d)

	return nil
}

// RemoveDataset from a container
// Returns true if the container is empty
// Only returns EntityNotFound
func (c *Container) RemoveDataset(datasetID string) (bool, error) {
	for _, dataset := range c.Datasets {
		if dataset.ID == datasetID {
			dataset.toDelete()
			return c.numActiveDatasets() == 0, nil
		}
	}
	return false, NewEntityNotFound("dataset", "id", datasetID, "Dataset "+datasetID+" in container "+c.URI)
}

// numActiveDatasets returns the number of referenced datasets in the container
// warning: does not count the "toDelete" datasets (even though they are not persisted yet)
func (c Container) numActiveDatasets() int {
	n := 0
	for _, d := range c.Datasets {
		if d.IsActive() {
			n++
		}
	}
	return n
}

// ToGcStorageClass returns the geocube storage class equivalent
func ToGcStorageClass(s string) (StorageClass, error) {
	switch strings.ToUpper(s) {
	case "STANDARD", "REGIONAL":
		return StorageClassSTANDARD, nil
	case "NEARLINE":
		return StorageClassINFREQUENT, nil
	case "COLDLINE":
		return StorageClassARCHIVE, nil
	case "ARCHIVE":
		return StorageClassDEEPARCHIVE, nil
	}
	return StorageClassUNDEFINED, NewValidationError("Unknown storage class: " + s)
}
