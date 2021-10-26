package image_test

import (
	"context"
	"os"
	"path"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/utils/affine"

	"github.com/airbusgeo/godal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var DatasetEquals = func(ds *godal.Dataset, wantedDsPath string) {
	pwd, _ := os.Getwd()
	wantedDs, err := godal.Open(path.Join(pwd, wantedDsPath))
	Expect(err).To(BeNil())

	defer wantedDs.Close()
	Expect(ds.Structure().SizeX).To(Equal(wantedDs.Structure().SizeX))
	Expect(ds.Structure().SizeY).To(Equal(wantedDs.Structure().SizeY))
	Expect(ds.Structure().DataType).To(Equal(wantedDs.Structure().DataType))
	Expect(ds.Structure().NBands).To(Equal(wantedDs.Structure().NBands))
	geo, err := ds.GeoTransform()
	Expect(err).To(BeNil())
	wantedGeo, err := wantedDs.GeoTransform()
	Expect(err).To(BeNil())
	Expect(geo).To(Equal(wantedGeo))
	Expect(ds.Projection()).To(Equal(wantedDs.Projection()))
	// read content
	returnedBmp, err := geocube.NewBitmapFromDataset(ds)
	Expect(err).To(BeNil())
	wantedBmp, err := geocube.NewBitmapFromDataset(wantedDs)
	Expect(err).To(BeNil())
	Expect(returnedBmp.Bytes).To(Equal(wantedBmp.Bytes))
}

var _ = Describe("CastDataset", func() {

	var (
		ctx                    = context.Background()
		fromPath               string
		fromDFormat, toDFormat geocube.DataMapping
		fromDs, returnedDs     *godal.Dataset
		returnedError          error
	)

	BeforeEach(func() {
		godal.RegisterAll()
	})

	JustBeforeEach(func() {
		pwd, _ := os.Getwd()
		fromDs, returnedError = godal.Open(path.Join(pwd, fromPath))
		Expect(returnedError).To(BeNil())
		returnedDs, returnedError = image.CastDataset(ctx, fromDs, fromDFormat, toDFormat, "")
	})

	JustAfterEach(func() {
		if returnedDs != nil {
			returnedDs.Close()
		}
		fromDs.Close()
	})

	var (
		itShouldNotReturnAnError = func() {
			It("should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}

		itShouldCastDataset = func(wantedDsPath string) {
			It("should cast the dataset", func() {
				DatasetEquals(returnedDs, wantedDsPath)
			})
		}
	)

	Context("to the same dataformat", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[0]
		})
		It("should returned an error", func() {
			Expect(returnedError).To(MatchError(image.ErrNoCastToPerform))
		})
	})

	Context("to rangeExt (toDformat=Id)", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[1]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[1])
	})

	Context("to another dataformat with same RangeExt", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[2]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[2])
	})

	Context("to another dataformat with another RangeExt", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[3]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[3])
	})

	Context("to another dataformat with an exponent", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[4]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[4])
	})

	Context("with an exponent to another dataformat", func() {
		BeforeEach(func() {
			fromPath = images[4]
			fromDFormat = imagesDFormat[4]
			toDFormat = imagesDFormat[0]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[0])
	})

	Context("to another dataformat with another RangeExt and an exponent", func() {
		BeforeEach(func() {
			fromPath = images[0]
			fromDFormat = imagesDFormat[0]
			toDFormat = imagesDFormat[5]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[5])
	})

	Context("with an exponent to another dataformat with another RangeExt", func() {
		BeforeEach(func() {
			fromPath = images[5]
			fromDFormat = imagesDFormat[5]
			toDFormat = imagesDFormat[6]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[6])
	})

	Context("with an exponent to another dataformat with the same exponent (same RangeExt.Min)", func() {
		BeforeEach(func() {
			fromPath = images[5]
			fromDFormat = imagesDFormat[5]
			toDFormat = imagesDFormat[7]
		})
		itShouldNotReturnAnError()
		itShouldCastDataset(images[7])
	})

})

var _ = Describe("MergeDataset", func() {

	var (
		ctx           = context.Background()
		fromPaths     []string
		fromDFormats  []geocube.DataMapping
		outDesc       image.GdalDatasetDescriptor
		returnedDs    *godal.Dataset
		returnedError error
	)

	BeforeEach(func() {
		godal.RegisterAll()
	})

	JustBeforeEach(func() {
		pwd, _ := os.Getwd()
		var datasets []*image.Dataset
		for i, fromPath := range fromPaths {
			datasets = append(datasets, &image.Dataset{
				URI:         path.Join(pwd, fromPath),
				DataMapping: fromDFormats[i],
			})
		}
		returnedDs, returnedError = image.MergeDatasets(ctx, datasets, &outDesc)
		Expect(returnedError).To(BeNil())
	})

	JustAfterEach(func() {
		if returnedDs != nil {
			returnedDs.Close()
		}
	})

	var (
		itShouldNotReturnAnError = func() {
			It("should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}

		itShouldMergeDatasets = func(wantedDsPath string) {
			It("should merge the datasets", func() {
				DatasetEquals(returnedDs, wantedDsPath)
			})
		}
	)

	Context("one dataset", func() {
		i := 8
		BeforeEach(func() {
			fromPaths = []string{images[i]}
			fromDFormats = []geocube.DataMapping{imagesDFormat[i]}
			outDesc = image.GdalDatasetDescriptor{
				WktCRS: "epsg:32632",
				PixToCRS: affine.Translation(460943.9866000000038184, 6255118.2874999996274710).
					Multiply(affine.Scale(200.198019801980081, -200.1990049751243816)),
				Width:  256,
				Height: 201,
				Bands:  1,

				Resampling:  geocube.ResamplingNEAR,
				DataMapping: imagesDFormat[i],
				ValidPixPc:  0,
			}
		})
		itShouldNotReturnAnError()
		itShouldMergeDatasets(images[i])
	})
	Context("two datasets with the same dataformat", func() {
		i1, i2, i3 := 8, 9, 11
		BeforeEach(func() {
			fromPaths = []string{images[i1], images[i2]}
			fromDFormats = []geocube.DataMapping{imagesDFormat[i1], imagesDFormat[i2]}
			outDesc = image.GdalDatasetDescriptor{
				WktCRS: "epsg:32632",
				PixToCRS: affine.Translation(460943.9866000000038184, 6255118.2874999996274710).
					Multiply(affine.Scale(200.198019801980081, -200.1990049751243816)),
				Width:       505,
				Height:      201,
				Bands:       1,
				Resampling:  geocube.ResamplingNEAR,
				DataMapping: imagesDFormat[i3],
				ValidPixPc:  0,
			}
		})
		itShouldNotReturnAnError()
		itShouldMergeDatasets(images[i3])
	})
	Context("two datasets with different dataformat", func() {
		i1, i2, i3 := 9, 10, 11
		BeforeEach(func() {
			fromPaths = []string{images[i1], images[i2]}
			fromDFormats = []geocube.DataMapping{imagesDFormat[i1], imagesDFormat[i2]}
			outDesc = image.GdalDatasetDescriptor{
				WktCRS: "epsg:32632",
				PixToCRS: affine.Translation(460943.9866000000038184, 6255118.2874999996274710).
					Multiply(affine.Scale(200.198019801980081, -200.1990049751243816)),
				Width:       505,
				Height:      201,
				Bands:       1,
				Resampling:  geocube.ResamplingNEAR,
				DataMapping: imagesDFormat[i3],
				ValidPixPc:  0,
			}
		})
		itShouldNotReturnAnError()
		itShouldMergeDatasets(images[i3])
	})
})
