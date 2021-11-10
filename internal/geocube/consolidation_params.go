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
	DFormat         DataFormat
	Exponent        float64
	Compression     Compression
	Overviews       bool
	DownsamplingAlg Resampling
	BandsInterleave bool
	StorageClass    StorageClass
}

// NewConsolidationParamsFromProtobuf creates a consolidation params from protobuf
// Only returns validationError
func NewConsolidationParamsFromProtobuf(pbp *pb.ConsolidationParams) (*ConsolidationParams, error) {
	dformat := NewDataFormatFromProtobuf(pbp.GetDformat())

	if pbp.GetDownsamplingAlg() == pb.Resampling_UNDEFINED {
		return nil, NewValidationError("Downsampling algorithm cannot be undefined")
	}

	c := ConsolidationParams{
		persistenceState: persistenceStateNEW,
		DFormat:          *dformat,
		Exponent:         pbp.GetExponent(),
		Compression:      Compression(pbp.GetCompression()),
		Overviews:        pbp.GetCreateOverviews(),
		DownsamplingAlg:  Resampling(pbp.GetDownsamplingAlg()),
		BandsInterleave:  pbp.GetBandsInterleave(),
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
		Dformat:         c.DFormat.ToProtobuf(),
		Exponent:        c.Exponent,
		CreateOverviews: c.Overviews,
		DownsamplingAlg: pb.Resampling(c.DownsamplingAlg),
		Compression:     pb.ConsolidationParams_Compression(c.Compression),
		BandsInterleave: c.BandsInterleave,
		StorageClass:    pb.StorageClass(c.StorageClass),
	}
}

func (c ConsolidationParams) validate() error {
	if !c.DFormat.validForPacking() {
		return NewValidationError("Data format is incorrect")
	}

	return nil
}
