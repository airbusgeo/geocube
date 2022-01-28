package geocube

//go:generate enumer -json -sql -type DatasetStatus -trimprefix DatasetStatus

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/twpayne/go-geom"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
)

type DatasetStatus int

const (
	DatasetStatusACTIVE DatasetStatus = iota
	DatasetStatusTODELETE
	DatasetStatusINACTIVE
)

type Dataset struct {
	persistenceState
	ID              string
	RecordID        string
	InstanceID      string
	ContainerURI    string
	ContainerSubDir string
	Bands           []int64
	DataMapping     DataMapping
	Status          DatasetStatus
	Shape           proj.Shape           ///< Valid shape in the crs of the dataset (may be smaller than the extent when there is nodata)
	GeogShape       proj.GeographicShape ///< Approximation of the valid shape in geographic coordinates
	GeomShape       proj.GeometricShape  ///< Approximation of the valid shape in 4326 coordinates
	Overviews       bool
}

// NewDatasetFromProtobuf creates a new dataset from protobuf
// Only returns validationError
func NewDatasetFromProtobuf(pbd *pb.Dataset, uri string) (*Dataset, error) {
	d := Dataset{
		persistenceState: persistenceStateNEW,
		ID:               uuid.New().String(),
		RecordID:         pbd.GetRecordId(),
		InstanceID:       pbd.GetInstanceId(),
		ContainerURI:     uri,
		ContainerSubDir:  pbd.GetContainerSubdir(),
		Bands:            pbd.GetBands(),
		DataMapping: DataMapping{
			DataFormat: *NewDataFormatFromProtobuf(pbd.GetDformat()),
			RangeExt:   Range{Min: pbd.GetRealMinValue(), Max: pbd.GetRealMaxValue()},
			Exponent:   pbd.GetExponent(),
		},
		Status: DatasetStatusACTIVE}

	if err := d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// IncompleteDatasetFromConsolidation returns a dataset partially initialized from the ConsolidationContainer of a consolidation task
// Only returns validationError
func IncompleteDatasetFromConsolidation(c *ConsolidationContainer, instanceID string) (*Dataset, error) {
	d := Dataset{
		persistenceState: persistenceStateNEW, // Set to New during the initialization
		InstanceID:       instanceID,
		ContainerURI:     c.URI,
		Bands:            make([]int64, c.BandsCount),
		DataMapping:      c.DatasetFormat,
		Status:           DatasetStatusINACTIVE,
		Overviews:        c.OverviewsMinSize != NO_OVERVIEW,
	}

	// Init GeoBBox
	transform := affine.Affine(c.Transform)
	p := proj.NewPolygonFromExtent(&transform, c.Width, c.Height)
	mp := geom.NewMultiPolygonFlat(geom.XY, p.FlatCoords(), [][]int{{10}})
	if err := d.SetShape(mp, c.CRS); err != nil {
		return nil, err
	}

	// Init bands
	for i := range d.Bands {
		d.Bands[i] = int64(i + 1)
	}

	d.persistenceState = persistenceStateUNKNOWN // Ensure that it cannot be persisted

	return &d, nil
}

// NewDatasetFromIncomplete creates a new dataset from dataset created by NewIncompleteDatasetFromConsolidation
// Only returns ValidationError
func NewDatasetFromIncomplete(d Dataset, consolidationRecord ConsolidationRecord, subdir string) (*Dataset, error) {
	d.persistenceState = persistenceStateNEW
	d.ID = uuid.New().String()
	d.RecordID = consolidationRecord.ID
	d.ContainerSubDir = subdir

	if consolidationRecord.ValidShape != nil {
		d.Shape = *consolidationRecord.ValidShape
		crs, err := proj.CRSFromEPSG(d.Shape.SRID())
		if err != nil {
			return nil, fmt.Errorf("NewDatasetFromIncomplete.%w", err)
		}
		if err := d.setGeomGeogShape(crs); err != nil {
			return nil, fmt.Errorf("NewDatasetFromIncomplete.%w", err)
		}
	}

	if err := d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// SetShape sets the shape in a given CRS and bounding boxes of a new dataset
func (d *Dataset) SetShape(shape *geom.MultiPolygon, crsS string) error {
	if !d.IsNew() {
		return NewValidationError("Set the shape of a dataset that is not new is forbidden")
	}

	crs, srid, err := proj.CRSFromUserInput(crsS)
	if err != nil {
		return NewValidationError("Invalid Crs: " + crsS)
	}

	defer crs.Close()
	d.Shape = proj.NewShape(srid, shape)

	return d.setGeomGeogShape(crs)
}

// SetShape sets the shape in a given CRS and bounding boxes of a new dataset
func (d *Dataset) setGeomGeogShape(crs *godal.SpatialRef) (err error) {
	d.GeomShape, err = proj.NewGeometricShapeFromShape(d.Shape, crs)
	if err != nil {
		return NewValidationError("Invalid crs or bbox: " + err.Error())
	}
	d.GeogShape, err = proj.NewGeographicShapeFromShape(d.Shape, crs)
	if err != nil {
		return NewValidationError("Invalid crs or bbox: " + err.Error())
	}
	return nil
}

// SetOverviews sets the overviews flag of a new dataset
func (d *Dataset) SetOverviews(hasOverviews bool) error {
	if !d.IsNew() {
		return NewValidationError("Set Overviews of a dataset that is not new is forbidden")
	}
	d.Overviews = hasOverviews
	return nil
}

// SetDataType sets the datatype flag of a new dataset
func (d *Dataset) SetDataType(dtype DType) error {
	if !d.IsNew() {
		return NewValidationError("Set DataType of a dataset that is not new is forbidden")
	}
	d.DataMapping.DType = dtype
	if err := d.DataMapping.validate(); err != nil {
		return NewValidationError("Invalid Dataset.DFormat (%s): %v", d.DataMapping.string(), err)
	}
	return nil
}

// ValidateWithVariable validates the instance using the full definition of the variable
// Only returns ValidationError
func (d *Dataset) ValidateWithVariable(v *Variable) error {
	if len(d.Bands) != len(v.Bands) {
		return NewValidationError("Wrong number of bands in dataset")
	}

	if !d.DataMapping.canCastTo(&v.DFormat) {
		return NewValidationError("Data format of dataset is incorrect as it cannot be cast to the data format of the variable")
	}

	if d.DataMapping.RangeExt.Min >= v.DFormat.Range.Max || d.DataMapping.RangeExt.Max <= v.DFormat.Range.Min {
		return NewValidationError(
			fmt.Sprintf("Range of external values of the dataset [%f,%f] does not intersect the range of values of the variable [%f,%f]",
				d.DataMapping.RangeExt.Min, d.DataMapping.RangeExt.Max, v.DFormat.Range.Min, v.DFormat.Range.Max))
	}
	return nil
}

// validate validates the fields of a Dataset
// Only returns ValidationError
func (d *Dataset) validate() error {
	if _, err := uuid.Parse(d.RecordID); err != nil {
		return NewValidationError("Invalid record id: " + d.RecordID)
	}
	if _, err := uuid.Parse(d.InstanceID); err != nil {
		return NewValidationError("Invalid instance id: " + d.InstanceID)
	}
	if err := d.DataMapping.validate(); err != nil {
		return NewValidationError("Invalid Dataset.DataMapping (%s): %v", d.DataMapping.string(), err)
	}
	return nil
}

// ToStorageClass returns the geocube sorage class equivalent
func ToStorageClass(s string) (StorageClass, error) {
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

// identicalTo compares two datasets and returns true if they are equals with exception of ID and persistentState
func (d *Dataset) identicalTo(d2 *Dataset) bool {
	return d.RecordID == d2.RecordID &&
		d.InstanceID == d2.InstanceID &&
		d.ContainerURI == d2.ContainerURI &&
		d.ContainerSubDir == d2.ContainerSubDir &&
		d.DataMapping.Equals(d2.DataMapping) &&
		d.Status == d2.Status &&
		d.Overviews == d2.Overviews &&
		d.Shape.Equal(&d2.Shape) &&
		d.GeogShape.Equal(&d2.GeogShape.Shape) &&
		d.GeomShape.Equal(&d2.GeomShape.Shape) &&
		utils.SliceInt64Equal(d.Bands, d2.Bands)
}

func (d *Dataset) GDALURI() string {
	return GDALURI(d.ContainerURI, d.ContainerSubDir)
}

func GDALURI(uri, subdir string) string {
	if subdir != "" {
		return fmt.Sprintf("%s:%s", subdir, uri)
	}
	return uri
}
