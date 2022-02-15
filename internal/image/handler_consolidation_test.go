package image_test

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/godal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HandleConsolidation", func() {

	var (
		// args
		ctx                     = context.Background()
		consolidationEventToUse *geocube.ConsolidationEvent
		workspace               string
		pwd, _                  = os.Getwd()

		returnedError error

		cogGenerator   = image.NewCogGenerator()
		mucogGenerator = image.NewMucogGenerator()

		handleConsolidation image.Handler
	)

	BeforeEach(func() {
		godal.RegisterAll()
		workspace = os.TempDir()
		handleConsolidation = image.NewHandleConsolidation(cogGenerator, mucogGenerator, os.TempDir(), 2)
	})

	var (
		itShouldNotReturnAnError = func() {
			It("it should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}

		itShouldCreateMucog = func() {
			It("it should create mucog", func() {

				fileInfo, _ := os.Stat("test_data/mucog.tif")
				Expect(fileInfo).NotTo(BeNil())

				dataset, err := godal.Open(path.Join(pwd, "test_data/mucog.tif"))
				Expect(err).To(BeNil())

				defer dataset.Close()
				Expect(dataset.Structure().SizeX).To(Equal(consolidationEventToUse.Container.Width))
				Expect(dataset.Structure().SizeY).To(Equal(consolidationEventToUse.Container.Height))
				Expect(dataset.GeoTransform()).To(Equal(consolidationEventToUse.Container.Transform))
				Expect(dataset.Structure().NBands).To(Equal(consolidationEventToUse.Container.BandsCount))
				Expect(dataset.Projection()).To(Equal(consolidationEventToUse.Container.CRS))
				Expect(dataset.Structure()).To(Equal(godal.DatasetStructure{
					BandStructure: godal.BandStructure{
						SizeX:      consolidationEventToUse.Container.Width,
						SizeY:      consolidationEventToUse.Container.Height,
						BlockSizeX: consolidationEventToUse.Container.BlockXSize,
						BlockSizeY: consolidationEventToUse.Container.BlockYSize,
						DataType:   consolidationEventToUse.Container.DatasetFormat.DType.ToGDAL(),
					},
					NBands: consolidationEventToUse.Container.BandsCount,
				}))
			})
		}

		itShouldReturnAnError = func(errMsg string) {
			It("it should not return an error", func() {
				Expect(returnedError).NotTo(BeNil())
				Expect(returnedError.Error()).To(Equal(errMsg))
			})
		}
		itShouldNotCreateMucog = func() {
			It("it should not create mucog", func() {
				fileInfo, _ := os.Stat("test_data/mucog.tif")
				Expect(fileInfo).To(BeNil())
			})
		}
	)

	Describe("Consolidate", func() {

		JustBeforeEach(func() {
			returnedError = handleConsolidation.Consolidate(ctx, consolidationEventToUse, workspace)
		})

		AfterEach(func() {
			os.Remove("test_data/mucog.tif")
		})

		Context("default with 1 record and 1 dataset", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent1Record
			})
			itShouldNotReturnAnError()
			itShouldCreateMucog()
		})

		Context("default with 1 record and 2 datasets", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent1Record2dataset
			})
			itShouldNotReturnAnError()
			itShouldCreateMucog()
		})

		Context("default with 2 records", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent2Record
			})
			itShouldNotReturnAnError()
			itShouldCreateMucog()
		})

		Context("default with other data format output", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent1RecordOtherDataFormat
			})
			itShouldNotReturnAnError()
			itShouldCreateMucog()
		})

		Context("when container URI is wrong", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent1Record
				consolidationEventToUse.Container.URI = "geocube-26628b52/d0b9702d-34c1-4ba0-a812-71247ddeccf3/865846.230447/6326946.956167/12195/12185/dc3845d2-d473-4ed9-a916-7fc88d044966/1.tif"
			})
			itShouldReturnAnError("failed to upload file on: geocube-26628b52/d0b9702d-34c1-4ba0-a812-71247ddeccf3/865846.230447/6326946.956167/12195/12185/dc3845d2-d473-4ed9-a916-7fc88d044966/1.tif : failed to parse uri: badly formatted storage uri")
			itShouldNotCreateMucog()
		})

		Context("when jobs is cancelled", func() {
			var (
				cancelledFilePath string
			)
			BeforeEach(func() {
				cancelledFilePath = path.Join(os.TempDir(), fmt.Sprintf("%s_%s", consolidationEventToUse.JobID, consolidationEventToUse.TaskID))
				if err := os.WriteFile(cancelledFilePath, []byte(""), 0777); err != nil {
					panic(err)
				}
			})
			itShouldReturnAnError("consolidation event is cancelled")
			itShouldNotCreateMucog()

			AfterEach(func() {
				if err := os.Remove(cancelledFilePath); err != nil {
					panic(err)
				}
			})
		})

		Context("when cogs is already usable", func() {
			BeforeEach(func() {
				consolidationEventToUse = ConsolidationEvent
			})
			itShouldNotReturnAnError()
			itShouldCreateMucog()
		})
	})

})
