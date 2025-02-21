package geocube

//go:generate go run github.com/dmarkham/enumer -json -sql -type StorageClass -trimprefix StorageClass

import (
	"fmt"
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
	ID           int
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
		return nil, NewValidationError("%v", err)
	}
	return &c, nil
}

// ToProtobuf converts a container to protobuf
func (c *Container) ToProtobuf() *pb.Container {
	container := pb.Container{
		Uri:      c.URI,
		Managed:  c.Managed,
		Datasets: make([]*pb.Dataset, 0, len(c.Datasets)),
	}
	for _, d := range c.Datasets {
		container.Datasets = append(container.Datasets, d.ToProtobuf())
	}
	return &container
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

	bs := utils.JoinInt64(d.Bands, "-")
	descError := func(reason string) string {
		return fmt.Sprintf("A dataset %s already refers to the container %s, the subdir '%s' and one of the bands among: %s", reason, c.URI, d.ContainerSubDir, bs)
	}

	for _, dataset := range c.Datasets {
		switch dataset.Status {
		case DatasetStatusTODELETE, DatasetStatusINACTIVE:
			return NewEntityAlreadyExists("dataset", "status", dataset.Status.String(), descError("with the status "+dataset.Status.String()))
		}
		if dataset.ContainerSubDir == d.ContainerSubDir && bandsIntersect(dataset.Bands, d.Bands) {
			// Check whether it's the same dataset
			if dataset.identicalTo(d) {
				return nil
			}
			reason := "which is different"
			if d.RecordID != dataset.RecordID {
				reason = "of another record (" + dataset.RecordID + ")"
			} else if d.InstanceID != dataset.InstanceID {
				reason = "of another instance (" + dataset.InstanceID + ")"
			} else if !d.DataMapping.Equals(dataset.DataMapping) {
				reason = "with another datamapping"
			} else if d.Status != dataset.Status {
				reason = "with a different internal status (" + dataset.Status.String() + ")"
			} else if !utils.SliceInt64Equal(d.Bands, dataset.Bands) {
				reason = "with a different set of bands"
			} else if d.Overviews != dataset.Overviews {
				reason = "with different overviews"
			} else if !d.Shape.Equal(&dataset.Shape) {
				reason = fmt.Sprintf("with different shape (%v != %v)", d.GeomShape.Coords(), dataset.GeomShape.Coords())
			}
			return NewEntityAlreadyExists("dataset", "subdir/bands", d.ContainerSubDir+"/"+bs, descError(reason))
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
	return StorageClassUNDEFINED, NewValidationError("Unknown storage class: %s", s)
}
