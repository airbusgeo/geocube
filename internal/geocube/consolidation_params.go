package geocube

import (
	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils/bitmap"
)

//go:generate go run github.com/dmarkham/enumer -json -sql -type Compression -trimprefix Compression

// Compression defines how the data is compressed in the file
type Compression int32

// Supported compression
const (
	CompressionNO Compression = iota
	CompressionLOSSLESS
	CompressionLOSSY
	CompressionCUSTOM // Compression is defined in CreationParams
)

// ConsolidationParams defines the parameters for the consolidation
type ConsolidationParams struct {
	persistenceState
	DFormat        DataFormat
	Exponent       float64
	Compression    Compression
	CreationParams Metadata
	ResamplingAlg  Resampling
	StorageClass   StorageClass
}

// Supported CreationParams
var SupportedCreationParams = []string{"PHOTOMETRIC", "PHOTOMETRIC_OVERVIEW", "COMPRESS", "COMPRESS_OVERVIEW", "JPEG_QUALITY", "JPEG_QUALITY_OVERVIEW", "PREDICTOR", "PREDICTOR_OVERVIEW", "ZLEVEL", "ZLEVEL_OVERVIEW", "ZSTD_LEVEL", "ZSTD_LEVEL_OVERVIEW", "MAX_Z_ERROR", "MAX_Z_ERROR_OVERVIEW", "JPEGTABLESMODE"}

// NewConsolidationParamsFromProtobuf creates a consolidation params from protobuf
// Only returns validationError
func NewConsolidationParamsFromProtobuf(pbp *pb.ConsolidationParams) (*ConsolidationParams, error) {
	dformat := NewDataFormatFromProtobuf(pbp.GetDformat())

	if pbp.GetResamplingAlg() == pb.Resampling_UNDEFINED {
		return nil, NewValidationError("Resampling algorithm cannot be undefined")
	}

	c := ConsolidationParams{
		persistenceState: persistenceStateNEW,
		DFormat:          *dformat,
		Exponent:         pbp.GetExponent(),
		Compression:      Compression(pbp.GetCompression()),
		CreationParams:   pbp.GetCreationParams(),
		ResamplingAlg:    Resampling(pbp.GetResamplingAlg()),
		StorageClass:     StorageClass(pbp.GetStorageClass()),
	}
	if c.CreationParams == nil {
		c.CreationParams = Metadata{}
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// ToProtobuf converts a consolidationParams to protobuf
func (c *ConsolidationParams) ToProtobuf() *pb.ConsolidationParams {
	return &pb.ConsolidationParams{
		Dformat:        c.DFormat.ToProtobuf(),
		Exponent:       c.Exponent,
		ResamplingAlg:  pb.Resampling(c.ResamplingAlg),
		Compression:    pb.ConsolidationParams_Compression(c.Compression),
		CreationParams: c.CreationParams,
		StorageClass:   pb.StorageClass(c.StorageClass),
	}
}

func (c ConsolidationParams) validate() error {
	if !c.DFormat.validForPacking() {
		return NewValidationError("Data format is incorrect")
	}
	if err := c.validateCompression(); err != nil {
		return err
	}
	if err := c.validateCreationParams(); err != nil {
		return err
	}

	return nil
}

func (c ConsolidationParams) addCreationParams(creationParams Metadata) {
	for k, v := range creationParams {
		c.CreationParams[k] = v
	}
}

func (c ConsolidationParams) validateCompression() error {
	switch c.Compression {
	case CompressionNO:
		return nil
	case CompressionLOSSY:
		switch c.DFormat.DType {
		case bitmap.DTypeUINT8, bitmap.DTypeINT8, bitmap.DTypeINT16, bitmap.DTypeUINT16, bitmap.DTypeINT32, bitmap.DTypeUINT32, bitmap.DTypeFLOAT32:
			c.addCreationParams(map[string]string{"COMPRESS": "LERC", "COMPRESS_OVERVIEW": "LERC", "MAX_Z_ERROR": "0.01", "MAX_Z_ERROR_OVERVIEW": "0.01"})
			return nil
		case bitmap.DTypeFLOAT64:
			c.addCreationParams(map[string]string{"COMPRESS": "LERC_ZSTD", "COMPRESS_OVERVIEW": "LERC_ZSTD", "MAX_Z_ERROR": "0.01", "MAX_Z_ERROR_OVERVIEW": "0.01"})
			return nil
		}

	case CompressionLOSSLESS:
		switch c.DFormat.DType {
		case bitmap.DTypeUINT8, bitmap.DTypeINT8, bitmap.DTypeINT16, bitmap.DTypeUINT16, bitmap.DTypeINT32, bitmap.DTypeUINT32, bitmap.DTypeFLOAT32:
			c.addCreationParams(map[string]string{"COMPRESS": "ZSTD", "COMPRESS_OVERVIEW": "ZSTD", "PREDICTOR": "2", "PREDICTOR_OVERVIEW": "2", "ZSTD_LEVEL": "0.01", "ZSTD_LEVEL_OVERVIEW": "0.01"})
			return nil
		case bitmap.DTypeFLOAT64:
			c.addCreationParams(map[string]string{"COMPRESS": "LERC_ZSTD", "COMPRESS_OVERVIEW": "LERC_ZSTD", "MAX_Z_ERROR": "0", "MAX_Z_ERROR_OVERVIEW": "0"})
			return nil
		}
	case CompressionCUSTOM:
		compress, ok := c.CreationParams["COMPRESS"]
		if !ok && c.Compression == CompressionCUSTOM {
			return NewValidationError("compression is CUSTOM, but creation_params COMPRESS is not defined")
		}
		if compress == "JPEG" {
			if c.DFormat.DType == bitmap.DTypeUINT8 || c.DFormat.DType == bitmap.DTypeINT8 {
				return nil
			}
		}
		return NewValidationError("compressionOption %s not supported for data type %s", compress, c.DFormat.DType.String())
	}

	return NewValidationError("compressionOption %s not supported for data type %s", c.Compression.String(), c.DFormat.DType.String())
}

func inSlide(v string, s []string) bool {
	for _, k := range s {
		if k == v {
			return true
		}
	}
	return false
}

func (c ConsolidationParams) validateCreationParams() error {
	for k := range c.CreationParams {
		if !inSlide(k, SupportedCreationParams) {
			return NewValidationError("unknown creationParams %s", k)
		}
	}
	return nil
}
