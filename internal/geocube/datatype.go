package geocube

//go:generate enumer -json -sql -type DType -trimprefix DType

import (
	"math"
	"strings"

	"github.com/airbusgeo/godal"
)

// DType is one of supported DataTypes for raster
type DType int

// Supported DataTypes
const (
	DTypeUNDEFINED DType = iota // reserved for bool
	DTypeUINT8
	DTypeUINT16
	DTypeUINT32
	DTypeINT8
	DTypeINT16
	DTypeINT32
	DTypeFLOAT32
	DTypeFLOAT64
	DTypeCOMPLEX64
)

var minValues = [...]float64{-math.MaxFloat64, 0, 0, 0, math.MinInt8, math.MinInt16, math.MinInt32, -math.MaxFloat32, -math.MaxFloat64,
	-math.MaxFloat64}

var maxValues = [...]float64{math.MaxFloat64, math.MaxUint8, math.MaxUint16, math.MaxUint32, math.MaxInt8, math.MaxInt16, math.MaxInt32,
	math.MaxFloat32, math.MaxFloat64, math.MaxFloat64}

func (dtype DType) minValue() float64 {
	return minValues[dtype]
}

func (dtype DType) maxValue() float64 {
	return maxValues[dtype]
}

func (dtype DType) canCastTo(dtypeTo DType) bool {
	if dtype == dtypeTo {
		return true
	}

	switch dtype {
	//	case DTypeBOOL:
	//		return dtypeTo == DTypeUINT8

	case DTypeCOMPLEX64:
		return dtypeTo == DTypeCOMPLEX64

	default:
		return dtypeTo != DTypeCOMPLEX64 // && dtypeTo != DTypeBOOL
	}
}

func (dtype DType) IsFloatingPointFormat() bool {
	switch dtype {
	case DTypeFLOAT32, DTypeFLOAT64, DTypeCOMPLEX64:
		return true
	}
	return false
}

func (dtype DType) ToGDAL() godal.DataType {
	switch dtype {
	//case DTypeBOOL:
	//	return gdal.Unknown
	case DTypeUINT8:
		return godal.Byte
	case DTypeUINT16:
		return godal.UInt16
	case DTypeUINT32:
		return godal.UInt32
	case DTypeINT8:
		return godal.Unknown
	case DTypeINT16:
		return godal.Int16
	case DTypeINT32:
		return godal.Int32
	case DTypeFLOAT32:
		return godal.Float32
	case DTypeFLOAT64:
		return godal.Float64
	case DTypeCOMPLEX64:
		return godal.CFloat32
	default:
		return godal.Unknown
	}
}

// DTypeFromGDal convert gdal.DataType to DType
func DTypeFromGDal(dtype godal.DataType) DType {
	switch dtype {
	case godal.Byte:
		return DTypeUINT8
	case godal.UInt16:
		return DTypeUINT16
	case godal.UInt32:
		return DTypeUINT32
	case godal.Int16:
		return DTypeINT16
	case godal.Int32:
		return DTypeINT32
	case godal.Float32:
		return DTypeFLOAT32
	case godal.Float64:
		return DTypeFLOAT64
	case godal.CFloat32:
		return DTypeCOMPLEX64
	default:
		return DTypeUNDEFINED
	}
	// TODO Handle GDalType: CInt16
	// TODO Handle GDalType: CInt32
	// TODO Handle GDalType: CFloat64
}

//DTypeFromString convert string dtype to DType
func DTypeFromString(dtype string) DType {
	switch strings.ToLower(dtype) {
	case "byte", "uint8":
		return DTypeUINT8
	case "uint16":
		return DTypeUINT16
	case "uint32":
		return DTypeUINT32
	case "int16":
		return DTypeINT16
	case "int32":
		return DTypeINT32
	case "float32":
		return DTypeFLOAT32
	case "float64":
		return DTypeFLOAT64
	case "cfloat64":
		return DTypeCOMPLEX64
	default:
		return DTypeUNDEFINED
	}
}

// Size returns the size of the dtype in bytes
func (dtype DType) Size() int {
	switch dtype {
	case /*DTypeBOOL,*/ DTypeUINT8, DTypeINT8:
		return 1
	case DTypeUINT16, DTypeINT16:
		return 2
	case DTypeUINT32, DTypeINT32, DTypeFLOAT32:
		return 4
	case DTypeFLOAT64, DTypeCOMPLEX64:
		return 8
	}
	panic("Unknown type")
}
