package geocube_test

import (
	"github.com/airbusgeo/geocube/internal/geocube"
	pb "github.com/airbusgeo/geocube/internal/pb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConsolidationParams", func() {

	var (
		// args
		consolidationParams    *geocube.ConsolidationParams
		pbConsolidationParams  = pb.ConsolidationParams{}
		expectedCreationParams map[string]string

		returnedError error
	)

	BeforeEach(func() {
		consolidationParams, returnedError = geocube.NewConsolidationParamsFromProtobuf(&pbConsolidationParams)
	})

	var (
		itShouldNotReturnAnError = func() {
			It("it should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}
		itShouldReturnAnError = func(errMsg string) {
			It("it should not return an error", func() {
				Expect(returnedError).NotTo(BeNil())
				Expect(returnedError.Error()).To(Equal(errMsg))
			})
		}

		itShouldCreateConsolidationParams = func() {
			It("it should create consolidation params", func() {
				Expect(consolidationParams.CreationParams).To(Equal(expectedCreationParams))
			})
		}
	)

	Describe("New", func() {
		Context("compression NO", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:     &pb.DataFormat{Dtype: pb.DataFormat_Float32},
					Compression: pb.ConsolidationParams_NO,
				}
				expectedCreationParams = map[string]string{}
			})
			itShouldNotReturnAnError()
			itShouldCreateConsolidationParams()
		})

		Context("compression LOSSLESS", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:     &pb.DataFormat{Dtype: pb.DataFormat_Float32},
					Compression: pb.ConsolidationParams_LOSSLESS,
				}
				expectedCreationParams = map[string]string{"COMPRESS": "ZSTD", "COMPRESS_OVERVIEW": "ZSTD", "PREDICTOR": "2", "PREDICTOR_OVERVIEW": "2", "ZSTD_LEVEL": "0.01", "ZSTD_LEVEL_OVERVIEW": "0.01"}
			})
			itShouldNotReturnAnError()
			itShouldCreateConsolidationParams()
		})

		Context("compression LOSSY", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:        &pb.DataFormat{Dtype: pb.DataFormat_Float32},
					Compression:    pb.ConsolidationParams_LOSSY,
					CreationParams: map[string]string{"COMPRESS": "JPEG", "JPEG_QUALITY": "2"},
				}
				expectedCreationParams = map[string]string{"COMPRESS": "LERC", "COMPRESS_OVERVIEW": "LERC", "MAX_Z_ERROR": "0.01", "MAX_Z_ERROR_OVERVIEW": "0.01", "JPEG_QUALITY": "2"}
			})
			itShouldNotReturnAnError()
			itShouldCreateConsolidationParams()
		})

		Context("compression LOSSY with COMPLEX", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:        &pb.DataFormat{Dtype: pb.DataFormat_Complex64},
					Compression:    pb.ConsolidationParams_LOSSY,
					CreationParams: map[string]string{},
				}
				expectedCreationParams = nil
			})
			itShouldReturnAnError("compressionOption LOSSY not supported for data type Complex64")
		})

		Context("compression JPEG with Float32", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:        &pb.DataFormat{Dtype: pb.DataFormat_Float32},
					Compression:    pb.ConsolidationParams_CUSTOM,
					CreationParams: map[string]string{"COMPRESS": "JPEG"},
				}
				expectedCreationParams = nil
			})
			itShouldReturnAnError("compressionOption JPEG not supported for data type Float32")
		})

		Context("compression JPEG with Int8", func() {
			BeforeEach(func() {
				pbConsolidationParams = pb.ConsolidationParams{
					Dformat:        &pb.DataFormat{Dtype: pb.DataFormat_UInt8},
					Compression:    pb.ConsolidationParams_CUSTOM,
					CreationParams: map[string]string{"COMPRESS": "JPEG"},
				}
				expectedCreationParams = map[string]string{"COMPRESS": "JPEG"}
			})
			itShouldNotReturnAnError()
			itShouldCreateConsolidationParams()
		})
	})

})
