package geocube

import (
	"fmt"
	"math"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
)

// DataFormat describes the internal format of a raster
type DataFormat struct {
	DType  DType
	NoData float64
	Range  Range
}

// DataMapping describes the mapping between an internal format and a external range
// vi in [Range.Min, Range.Max], ve in [RangeExt.Min, RangeExt.Max] are linked as follows:
// ve = RangeExt.Min + (RangeExt.Max - RangeExt.Min) * ((vi - Range.Min)/(Range.Max - Range.Min))^Exponent
type DataMapping struct {
	DataFormat
	RangeExt Range   // External range maps to Internal range
	Exponent float64 // For non-linear mapping from RangeInt to RangeExt
}

// Range of values
type Range struct {
	Min, Max float64
}

func (r Range) Interval() float64 {
	return r.Max - r.Min
}

func NewDataFormatFromProtobuf(pbdf *pb.DataFormat) *DataFormat {
	return &DataFormat{
		DType:  DType(pbdf.GetDtype()),
		NoData: pbdf.GetNoData(),
		Range:  Range{Min: pbdf.GetMinValue(), Max: pbdf.GetMaxValue()},
	}
}

func (df DataFormat) ToProtobuf() *pb.DataFormat {
	return &pb.DataFormat{
		Dtype:    pb.DataFormat_Dtype(df.DType),
		NoData:   df.NoData,
		MinValue: df.Range.Min,
		MaxValue: df.Range.Max}
}

func (r Range) validate() error {
	if r.Min >= r.Max {
		return fmt.Errorf("min must be stricly lower than max")
	}
	return nil
}

func (dm DataMapping) validate() error {
	if err := dm.DataFormat.validate(); err != nil {
		return err
	}

	if err := dm.RangeExt.validate(); err != nil {
		return err
	}

	if dm.Exponent <= 0 {
		return NewValidationError("invalid exponent (must be strictly positive)")
	}

	return nil
}

func (df DataFormat) validate() error {
	minValue := df.DType.minValue()
	maxValue := df.DType.maxValue()

	if !(df.Range.Min >= minValue && df.Range.Max <= maxValue) {
		return fmt.Errorf("min/max value are out of bounds [%f, %f]", minValue, maxValue)
	}

	if err := df.Range.validate(); err != nil {
		return err
	}

	if !math.IsNaN(df.NoData) && df.NoData > df.Range.Min && df.NoData < df.Range.Max {
		return fmt.Errorf("noData value cannot be strictly between min and max values")
	}

	if !math.IsNaN(df.NoData) && (df.NoData < minValue || df.NoData > maxValue) {
		return fmt.Errorf("noData value (%f) is not supported by the data type (%s). If nodata is not defined, set it to NaN", df.NoData, df.DType.String())
	}

	return nil
}

// NoDataDefined returns True if the user has defined a NoData value.
// When NoData is not defined, its value is NaN, whatever the DataType
func (df DataFormat) NoDataDefined() bool {
	return !math.IsNaN(df.NoData) || df.DType.IsFloatingPointFormat()
}

func (dm DataMapping) Equals(dm2 DataMapping) bool {
	return dm.DataFormat.Equals(dm2.DataFormat) &&
		dm.RangeExt == dm2.RangeExt &&
		dm.Exponent == dm2.Exponent
}

func (df DataFormat) Equals(df2 DataFormat) bool {
	return df.DType == df2.DType &&
		df.Range == df2.Range &&
		(df.NoData == df2.NoData || (math.IsNaN(df.NoData) && math.IsNaN(df2.NoData)))
}

func (df DataFormat) canCastTo(dTo *DataFormat) bool {
	return df.DType.canCastTo(dTo.DType)
}

func (df DataFormat) validForPacking() bool {
	return true //df.DType != DTypeBOOL
}

func (df DataFormat) string() string {
	return fmt.Sprintf("(%s %s, nodata:%s)", df.DType.String(), df.Range.string(), utils.F64ToS(df.NoData))
}

func (r Range) string() string {
	return fmt.Sprintf("[%s -> %s]", utils.F64ToS(r.Min), utils.F64ToS(r.Max))
}
