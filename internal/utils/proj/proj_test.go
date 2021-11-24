package proj_test

import (
	"math"

	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-geom"
)

var _ = Describe("Ring", func() {
	var err error
	var pixToCRS *affine.Affine
	var ring, ringExpected proj.Ring

	var (
		itShouldBeEqual = func(ring, expected *proj.Ring) {
			It("it should be equal", func() {
				Expect(ring.Equal(expected)).To(BeTrue())
			})
		}

		itShouldNotReturnError = func() {
			It("it should not return error", func() {
				Expect(err).To(BeNil())
			})
		}
	)

	BeforeEach(func() {
		pixToCRS = affine.Translation(453120, 5338560).Multiply(affine.Scale(10, -10))
		ringExpected = proj.NewRingFlat(4326, []float64{453120, 5334400, 453120, 5338560, 499520, 5338560, 499520, 5334400, 453120, 5334400})
	})

	Describe("NewRingFromExtent", func() {
		JustBeforeEach(func() {
			ring = proj.NewRingFromExtent(pixToCRS, 4640, 416, 4326)
		})

		Context("create ring", func() {
			itShouldNotReturnError()
			itShouldBeEqual(&ring, &ringExpected)
		})
	})
})

func truncate(slice []float64, precision int) []float64 {
	output := math.Pow(10, float64(precision))
	res := make([]float64, len(slice))
	for i := range slice {
		res[i] = float64(int(slice[i]*output)) / output
	}
	return res
}

var _ = Describe("Shape", func() {
	var err error
	var geomShape, gms proj.GeometricShape
	var geogShape, ggs proj.GeographicShape
	var shape32630 proj.Shape
	var crs *godal.SpatialRef

	var (
		itShouldBeEqual = func(shape, expected *proj.Shape) {
			It("it should be equal", func() {
				Expect(shape.SRID()).To(Equal(expected.SRID()))
				Expect(truncate(shape.FlatCoords(), 10)).To(Equal(truncate(expected.FlatCoords(), 10)))
			})
		}

		itShouldNotReturnError = func() {
			It("it should not return error", func() {
				Expect(err).To(BeNil())
			})
		}
	)

	BeforeEach(func() {
		mp := geom.NewMultiPolygon(geom.XY)
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{852835, 4842077, 863531, 4840218, 860880, 4833605, 852499, 4833757, 852835, 4842077}, []int{10}))
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{-482825, 6270337, 1804924, 6565717, 1943927, 3258617, -1397924, 4144758, -482825, 6270337}, []int{10}))
		mp.SetSRID(32630)
		shape32630 = proj.Shape{*mp}

		mp = geom.NewMultiPolygon(geom.XY)
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{1.3748665564675484, 43.64792634710127, 1.5058403390677146, 43.62609258997801, 1.4686768515566098, 43.56801884119599, 1.3652960374744307, 43.573389234301054, 1.3748665564675484, 43.64792634710127}, []int{10}))
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{-18.658950044690343, 55.57172477118127, -9.780684371963316, 57.05708065821401, -0.28440430479784556, 57.875001670424595, 9.429326625880611, 57.9582583598197, 18.893370030537877, 57.308939432730774, 15.858895801315843, 50.25146071559958, 13.875872641077368, 43.098533289930906, 11.687050875950385, 28.644776608765394, 3.3873712764612716, 31.296735169738305, -5.441615925756458, 33.430572186260825, -14.600743461281139, 34.893131241906225, -23.808028524867566, 35.59340069548139, -18.658950044690343, 55.57172477118127}, []int{36}))
		mp.SetSRID(4326)
		geomShape = proj.GeometricShape{proj.Shape{*mp}}

		mp = geom.NewMultiPolygon(geom.XY)
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{1.3748665564675484, 43.64792634710127, 1.5058403390677146, 43.62609258997801, 1.4686768515566098, 43.56801884119599, 1.3652960374744307, 43.573389234301054, 1.3748665564675484, 43.64792634710127}, []int{10}))
		mp.Push(geom.NewPolygonFlat(geom.XY, []float64{-18.658950044690343, 55.57172477118127, 18.893370030537877, 57.308939432730774, 13.875872641077368, 43.098533289930906, 11.687050875950385, 28.644776608765394, -23.808028524867566, 35.59340069548139, -18.658950044690343, 55.57172477118127}, []int{22}))
		mp.SetSRID(4326)
		geogShape = proj.GeographicShape{proj.Shape{*mp}}

		crs, err = proj.CRSFromEPSG(32630)
		Expect(err).To(BeNil())
	})

	Describe("NewGeometricShapeFromShape", func() {
		JustBeforeEach(func() {
			gms, err = proj.NewGeometricShapeFromShape(shape32630, crs)
		})

		Context("create geometric shape", func() {
			itShouldNotReturnError()
			itShouldBeEqual(&gms.Shape, &geomShape.Shape)
		})
	})

	Describe("NewGeographicShapeFromShape", func() {
		JustBeforeEach(func() {
			ggs, err = proj.NewGeographicShapeFromShape(shape32630, crs)
		})

		Context("create geographic shape", func() {
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})
	})
})
