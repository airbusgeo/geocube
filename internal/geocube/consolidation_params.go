package geocube

import (
	pb "github.com/airbusgeo/geocube/internal/pb"
)

//go:generate enumer -json -sql -type Compression -trimprefix Compression

// Compression defines how the data is compressed in the file
type Compression int32

// Supported compression
const (
	CompressionNO Compression = iota
	CompressionLOSSLESS
	CompressionLOSSY
)

// ConsolidationParams defines the parameters for the consolidation
type ConsolidationParams struct {
	persistenceState
	DFormat       DataFormat
	Exponent      float64
	Compression   Compression
	ResamplingAlg Resampling
	StorageClass  StorageClass
}

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
		ResamplingAlg:    Resampling(pbp.GetResamplingAlg()),
		StorageClass:     StorageClass(pbp.GetStorageClass()),
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// ToProtobuf converts a consolidationParams to protobuf
func (c *ConsolidationParams) ToProtobuf() *pb.ConsolidationParams {
	return &pb.ConsolidationParams{
		Dformat:       c.DFormat.ToProtobuf(),
		Exponent:      c.Exponent,
		ResamplingAlg: pb.Resampling(c.ResamplingAlg),
		Compression:   pb.ConsolidationParams_Compression(c.Compression),
		StorageClass:  pb.StorageClass(c.StorageClass),
	}
}

func (c ConsolidationParams) validate() error {
	if !c.DFormat.validForPacking() {
		return NewValidationError("Data format is incorrect")
	}

	return nil
}
