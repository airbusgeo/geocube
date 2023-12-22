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

var _ = Describe("GeographicShape", func() {
	var err error
	var geogShape, ggs proj.GeographicShape
	var shape proj.Shape
	var poly, geogPoly *geom.Polygon
	var srid int
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

	Describe("New GeographicShape From Shape", func() {
		JustBeforeEach(func() {
			mp := geom.NewMultiPolygon(geom.XY)
			mp.Push(poly)
			mp.SetSRID(32630)
			shape = proj.Shape{*mp}

			mp = geom.NewMultiPolygon(geom.XY)
			mp.Push(geogPoly)
			mp.SetSRID(4326)
			geogShape = proj.GeographicShape{proj.Shape{*mp}}

			crs, err = proj.CRSFromEPSG(srid)
			Expect(err).To(BeNil())
			ggs, err = proj.NewGeographicShapeFromShape(shape, crs)
		})

		Context("create 32701 shape over meridian 180", func() {

			BeforeEach(func() {
				srid = 32701
				poly = geom.NewPolygonFlat(geom.XY, []float64{100000, 7590000, 100000, 7700000, 200000, 7700000, 200000, 7590000, 100000, 7590000}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{179.1337407477, -21.7485383988, 179.1595683063, -20.7569050097, 180.1186085085, -20.7756874907, 180.099204994, -21.7683053952, 179.1337407477, -21.7485383988}, []int{10})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 3857 shape over meridian 180", func() {

			BeforeEach(func() {
				srid = 3857
				poly = geom.NewPolygonFlat(geom.XY, []float64{20000000, -17000000, 21000000, -17000000, 21000000, 17000000, 20000000, 17000000, 20000000, -17000000}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{179.6630568239, -82.0401602032, 184.1546332445, -82.0401602032, 188.64620966501, -82.0401602032, 188.64620966501, 82.0401602032, 184.1546332445, 82.0401602032, 179.6630568239, 82.0401602032, 179.6630568239, -82.0401602032}, []int{14})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 3857 shape over meridian -180", func() {

			BeforeEach(func() {
				srid = 3857
				poly = geom.NewPolygonFlat(geom.XY, []float64{-21000000, -17000000, -20000000, -17000000, -20000000, 17000000, -21000000, 17000000, -21000000, -17000000}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{171.3537903349, -82.0401602032, 175.8453667554, -82.0401602032, 180.336943176, -82.0401602032, 180.336943176, 82.0401602032, 175.8453667554, 82.0401602032, 171.3537903349, 82.0401602032, 171.3537903349, -82.0401602032}, []int{14})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 3857 worldwide shape", func() {

			BeforeEach(func() {
				srid = 3857
				poly = geom.NewPolygonFlat(geom.XY, []float64{-20000000, -17000000, 20000000, -17000000, 20000000, 17000000, -20000000, 17000000, -20000000, -17000000}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-179.6630568239, -82.0401602032, -157.2051747209, -82.0401602032, -134.7472926179, -82.0401602032, -112.2894105149, -82.0401602032, -89.8315284119, -82.0401602032, -67.3736463089, -82.0401602032, -44.91576420591, -82.0401602032, -22.4578821029, -82.0401602032, 0, -82.0401602032, 22.4578821029, -82.0401602032, 44.91576420591, -82.0401602032, 67.3736463089, -82.0401602032, 89.8315284119, -82.0401602032, 112.2894105149, -82.0401602032, 134.7472926179, -82.0401602032, 157.2051747209, -82.0401602032, 179.6630568239, -82.0401602032, 179.6630568239, 82.0401602032, 157.2051747209, 82.0401602032, 134.7472926179, 82.0401602032, 112.2894105149, 82.0401602032, 89.8315284119, 82.0401602032, 67.3736463089, 82.0401602032, 44.91576420591, 82.0401602032, 22.4578821029, 82.0401602032, 0, 82.0401602032, -22.4578821029, 82.0401602032, -44.91576420591, 82.0401602032, -67.3736463089, 82.0401602032, -89.8315284119, 82.0401602032, -112.2894105149, 82.0401602032, -134.7472926179, 82.0401602032, -157.2051747209, 82.0401602032, -179.6630568239, 82.0401602032, -179.6630568239, -82.0401602032}, []int{70})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 3857 strange worldwide shape", func() {

			BeforeEach(func() {
				srid = 3857
				poly = geom.NewPolygonFlat(geom.XY, []float64{-20000000, -17000000, 19000000, 0, -1000000, -17000000, 20000000, -17000000, 20000000, 17000000, -20000000, 17000000, -20000000, -17000000}, []int{14})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-179.6630568239, -82.0401602032, -135.870186723, -78.90982629, -92.0773166222, -74.5703853942, -48.2844465214, -68.5913113239, -26.388011471, -64.8252828321, -4.4915764205, -60.44727889, 17.4048586298, -55.3878158714, 39.3012936802, -49.5866717334, 61.1977287306, -43.0034697947, 83.094163781, -35.6312510832, 104.9905988314, -27.5113123375, 126.8870338818, -18.7455386529, 170.6799039827, 0, 148.2220218797, -18.7455386529, 125.7641397767, -35.6312510832, 103.3062576737, -49.5866717334, 80.8483755707, -60.44727889, 58.3904934677, -68.5913113239, 35.9326113647, -74.5703853942, 13.4747292617, -78.90982629, -8.9831528411, -82.0401602032, 14.5976233669, -82.0401602032, 38.178399575, -82.0401602032, 61.7591757832, -82.0401602032, 85.3399519913, -82.0401602032, 108.9207281994, -82.0401602032, 132.5015044076, -82.0401602032, 156.0822806157, -82.0401602032, 179.6630568239, -82.0401602032, 179.6630568239, 82.0401602032, 157.2051747209, 82.0401602032, 134.7472926179, 82.0401602032, 112.2894105149, 82.0401602032, 89.8315284119, 82.0401602032, 67.3736463089, 82.0401602032, 44.91576420591, 82.0401602032, 22.4578821029, 82.0401602032, 0, 82.0401602032, -22.4578821029, 82.0401602032, -44.91576420591, 82.0401602032, -67.3736463089, 82.0401602032, -89.8315284119, 82.0401602032, -112.2894105149, 82.0401602032, -134.7472926179, 82.0401602032, -157.2051747209, 82.0401602032, -179.6630568239, 82.0401602032, -179.6630568239, -82.0401602032}, []int{94})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 3857 bigger than worldwide shape", func() {

			BeforeEach(func() {
				srid = 3857
				poly = geom.NewPolygonFlat(geom.XY, []float64{-20000000, -17000000, 21000000, -17000000, 21000000, 17000000, -20000000, 17000000, -20000000, -17000000}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-179.6630568239, -82.0401602032, -156.6437276683, -82.0401602032, -133.6243985127, -82.0401602032, -110.6050693572, -82.0401602032, -87.5857402016, -82.0401602032, -64.566411046, -82.0401602032, -41.5470818905, -82.0401602032, -18.5277527349, -82.0401602032, 4.4915764205, -82.0401602032, 27.5109055761, -82.0401602032, 50.5302347317, -82.0401602032, 73.5495638872, -82.0401602032, 96.5688930428, -82.0401602032, 119.5882221984, -82.0401602032, 142.6075513539, -82.0401602032, 165.6268805095, -82.0401602032, 188.64620966509, -82.0401602032, 188.64620966509, 82.0401602032, 165.6268805095, 82.0401602032, 142.6075513539, 82.0401602032, 119.5882221984, 82.0401602032, 96.5688930428, 82.0401602032, 73.5495638872, 82.0401602032, 50.5302347317, 82.0401602032, 27.5109055761, 82.0401602032, 4.4915764205, 82.0401602032, -18.5277527349, 82.0401602032, -41.5470818905, 82.0401602032, -64.566411046, 82.0401602032, -87.5857402016, 82.0401602032, -110.6050693572, 82.0401602032, -133.6243985127, 82.0401602032, -156.6437276683, 82.0401602032, -179.6630568239, 82.0401602032, -179.6630568239, -82.0401602032}, []int{70})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 4326 shape over meridian 180", func() {

			BeforeEach(func() {
				srid = 4326
				poly = geom.NewPolygonFlat(geom.XY, []float64{170, 85, 170, -85, 190, -85, 190, 85, 170, 85}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{170, 85, 170, -85, 175, -85, 180, -85, 185, -85, 190, -85, 190, 85, 185, 85, 180, 85, 175, 85, 170, 85}, []int{22})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 4326 shape over meridian -180", func() {

			BeforeEach(func() {
				srid = 4326
				poly = geom.NewPolygonFlat(geom.XY, []float64{-190, 85, -190, -85, -170, -85, -170, 85, -190, 85}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-190, 85, -190, -85, -185, -85, -180, -85, -175, -85, -170, -85, -170, 85, -175, 85, -180, 85, -185, 85, -190, 85}, []int{22})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 4326 worldwide shape", func() {

			BeforeEach(func() {
				srid = 4326
				poly = geom.NewPolygonFlat(geom.XY, []float64{-180, 85, -180, -85, 180, -85, 180, 85, -180, 85}, []int{10})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-180, 85, -180, -85, -157.5, -85, -135, -85, -112.5, -85, -90, -85, -67.5, -85, -45, -85, -22.5, -85, 0, -85, 22.5, -85, 45, -85, 67.5, -85, 90, -85, 112.5, -85, 135, -85, 157.5, -85, 180, -85, 180, 85, 157.5, 85, 135, 85, 112.5, 85, 90, 85, 67.5, 85, 45, 85, 22.5, 85, 0, 85, -22.5, 85, -45, 85, -67.5, 85, -90, 85, -112.5, 85, -135, 85, -157.5, 85, -180, 85}, []int{70})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})

		Context("create 4326 strange worldwide shape", func() {

			BeforeEach(func() {
				srid = 4326
				poly = geom.NewPolygonFlat(geom.XY, []float64{-180, 85, -180, -85, 170, 0, -10, -85, 180, -85, 180, 85, -180, 85}, []int{14})
				geogPoly = geom.NewPolygonFlat(geom.XY, []float64{-180, 85, -180, -85, -158.125, -79.6875, -136.25, -74.375, -114.375, -69.0625, -92.5, -63.75, -70.625, -58.4375, -48.75, -53.125, -26.875, -47.8125, -5, -42.5, 16.875, -37.1875, 38.75, -31.875, 60.625, -26.5625, 82.5, -21.25, 126.25, -10.625, 170, 0, 125, -21.25, 102.5, -31.875, 80, -42.5, 57.5, -53.125, 35, -63.75, 12.5, -74.375, -10, -85, 13.75, -85, 37.5, -85, 61.25, -85, 85, -85, 108.75, -85, 132.5, -85, 156.25, -85, 180, -85, 180, 85, 157.5, 85, 135, 85, 112.5, 85, 90, 85, 67.5, 85, 45, 85, 22.5, 85, 0, 85, -22.5, 85, -45, 85, -67.5, 85, -90, 85, -112.5, 85, -135, 85, -157.5, 85, -180, 85}, []int{96})
			})
			itShouldNotReturnError()
			itShouldBeEqual(&ggs.Shape, &geogShape.Shape)
		})
	})
})
