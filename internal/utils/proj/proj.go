package proj

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"database/sql/driver"

	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/godal"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/ewkbhex"
)

const (
	RadToDeg = 180 / math.Pi
	DegToRad = math.Pi / 180
)

// CreateLonLatProj create a CoordinateTransform from/to the geographic lon/lat coordinates
func CreateLonLatProj(crs *godal.SpatialRef, inverse bool) (*godal.Transform, error) {
	lonlatCRS, err := CRSFromEPSG(4326)
	if err != nil {
		return nil, fmt.Errorf("CreateLonLatProj.%w", err)
	}

	var tr *godal.Transform
	if inverse {
		tr, err = godal.NewTransform(crs, lonlatCRS)
	} else {
		tr, err = godal.NewTransform(lonlatCRS, crs)
	}
	if err != nil {
		return nil, fmt.Errorf("CreateLonLatProj: %w", err)
	}
	runtime.SetFinalizer(tr, func(tr *godal.Transform) { tr.Close() })
	return tr, nil
}

// CRSFromUserInput initialize a crs from epsg, proj4 or Wkt format
// Return the SRID if known
func CRSFromUserInput(input string) (*godal.SpatialRef, int, error) {
	var err error
	var crs *godal.SpatialRef
	if epsg, err := strconv.Atoi(input); err == nil {
		crs, err = godal.NewSpatialRefFromEPSG(epsg)
		return crs, epsg, err
	}
	if strings.HasPrefix(strings.ToLower(input), "epsg") {
		epsg, err := strconv.Atoi(input[5:])
		if err != nil {
			return nil, 0, err
		}
		crs, err = godal.NewSpatialRefFromEPSG(epsg)
		return crs, epsg, err
	}
	if strings.HasPrefix(input, "+") {
		crs, err = godal.NewSpatialRefFromProj4(input)
		return crs, Srid(crs), err
	}
	crs, err = godal.NewSpatialRefFromWKT(input)
	return crs, Srid(crs), err
}

var crsEPSG map[int]*godal.SpatialRef = map[int]*godal.SpatialRef{}
var crsEPSGLock sync.Mutex

// CRSFromEPSG initialize a crs from epsg (only once per epsg)
// DO NOT release the crs (it is kept for further uses)
func CRSFromEPSG(epsg int) (*godal.SpatialRef, error) {
	crsEPSGLock.Lock()
	defer crsEPSGLock.Unlock()

	if crs, ok := crsEPSG[epsg]; ok && crs != nil {
		return crs, nil
	}

	var err error
	crsEPSG[epsg], err = godal.NewSpatialRefFromEPSG(epsg)
	if err != nil {
		return nil, fmt.Errorf("CRSFromEPSG: %w", err)
	}
	runtime.SetFinalizer(crsEPSG[epsg], func(crs *godal.SpatialRef) { crs.Close() })
	return crsEPSG[epsg], nil
}

// SRID returns the SRID from the crs or 0 if not found
// Warning : this function is not reliable...
func Srid(crs *godal.SpatialRef) int {
	if crs == nil {
		return 0
	}
	entities := []string{"PROJCS", "PROJCS", "LOCAL_CS", "GEOGCS"}
	for i, entity := range entities {
		if crs.AuthorityName(entity) == "EPSG" {
			if res, err := strconv.Atoi(crs.AuthorityCode(entity)); err == nil {
				return res
			}
		}
		if i == 0 {
			crs.AutoIdentifyEPSG()
		}
	}
	return 0
}

// FlatCoordToXY splits flat into two arrays x, y
func FlatCoordToXY(flat []float64) (x []float64, y []float64) {
	n := len(flat) / 2
	x = make([]float64, n)
	y = make([]float64, n)
	for i, j := 0, 0; i < n; i, j = i+1, j+2 {
		x[i], y[i] = flat[j], flat[j+1]
	}
	return x, y
}

// XYToFlatCoord merge two arrays x, y into one, interleaving coordinates.
func XYToFlatCoord(x []float64, y []float64) []float64 {
	n := len(x)
	flat := make([]float64, 2*n)
	for i, j := 0, 0; i < n; i, j = i+1, j+2 {
		flat[j], flat[j+1] = x[i], y[i]
	}
	return flat
}

/*******************************************************************/
/*                            SHAPES                               */
/*******************************************************************/

// Shape is a XY-Multipolygon implementing Scan & Value
// Warning: SRID is not reliable
type Shape struct {
	geom.MultiPolygon
}

// NewShape create a new shape
func NewShape(srid int, mp *geom.MultiPolygon) Shape {
	p := mp.Clone()
	p.SetSRID(srid)
	return Shape{*p}
}

func (shape Shape) MarshalBinary() ([]byte, error) {
	value, err := ewkbhex.Encode(&shape.MultiPolygon, ewkbhex.NDR)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shape as binary: %w", err)
	}
	return []byte(value), nil
}

func (shape *Shape) UnmarshalBinary(data []byte) error {
	g, err := ewkbhex.Decode(string(data))
	if err != nil {
		return fmt.Errorf("failed to decode binary data as geometry: %w", err)
	}
	geom, ok := g.(*geom.MultiPolygon)
	if !ok {
		return fmt.Errorf("shape.UnmarshalBinary: data is not a multipolygon")
	}
	shape.MultiPolygon = *geom
	return nil
}

// GeographicShape is a geographic polygon of lon/lat (following geodesic lines)
type GeographicShape struct{ Shape }

// GeometricShape is a polygon of lon/lat (4326)
type GeometricShape struct{ Shape }

// Scan implements the sql.Scanner interface.
func (shape *Shape) Scan(src interface{}) error {
	if src == nil {
		*shape = Shape{}
		return nil
	}

	switch src := src.(type) {
	case []uint8:
		g, err := ewkbhex.Decode(string(src))
		if err != nil {
			return err
		}
		geom, ok := g.(*geom.MultiPolygon)
		if !ok {
			return fmt.Errorf("shape.Scan: data is not a multipolygon")
		}
		shape.MultiPolygon = *geom
		return nil
	}

	return fmt.Errorf("cannot convert %T to Shape", src)
}

// Value implements the driver.Valuer interface.
func (shape *Shape) Value() (driver.Value, error) {
	return ewkbhex.Encode(&shape.MultiPolygon, ewkbhex.NDR)
}

// Equal returns true if the shapes have the same SRID and the same FlatCoords
func (shape *Shape) Equal(shape2 *Shape) bool {
	return shape.SRID() == shape2.SRID() && utils.SliceFloat64Equal(shape.FlatCoords(), shape2.FlatCoords())
}

// NewGeometricShapeFromShape creates a shape in 4326 lon/lat coordinates that covers the shape in planar crs
func NewGeometricShapeFromShape(shape Shape, crs *godal.SpatialRef) (g GeometricShape, err error) {
	p, err := shape.cloneTo4326(crs, false)
	if err != nil {
		return GeometricShape{}, fmt.Errorf("NewGeometricShapeFromShape.%w", err)
	}
	return GeometricShape{*p}, nil
}

// NewGeographicShapeFromShape creates a shape in geographic coordinates that covers the shape in planar crs
func NewGeographicShapeFromShape(shape Shape, crs *godal.SpatialRef) (GeographicShape, error) {
	p, err := shape.cloneTo4326(crs, true)
	if err != nil {
		return GeographicShape{}, fmt.Errorf("NewGeographicShapeFromShape.%w", err)
	}
	return GeographicShape{*p}, nil
}

/*******************************************************************/
/*                            RINGS                                */
/*******************************************************************/

type Ring struct {
	geom.LinearRing
}

// GeographicShape is a geographic polygon of lon/lat (following geodesic lines)
type GeographicRing struct{ Ring }

// GeometricShape is a polygon of lon/lat (4326)
type GeometricRing struct{ Ring }

// Equal returns true if the shapes have the same SRID and the same FlatCoords
func (ring *Ring) Equal(ring2 *Ring) bool {
	return ring.SRID() == ring2.SRID() && utils.SliceFloat64Equal(ring.FlatCoords(), ring2.FlatCoords())
}

// NewRingFlat create a new ring
func NewRingFlat(srid int, flatCoords []float64) Ring {
	r := geom.NewLinearRingFlat(geom.XY, flatCoords)
	r.SetSRID(srid)
	return Ring{*r}
}

// Value implements the driver.Valuer interface.
func (ring *Ring) Value() (driver.Value, error) {
	polygon := geom.NewPolygonFlat(ring.Layout(), ring.FlatCoords(), []int{len(ring.FlatCoords())})
	polygon.SetSRID(ring.SRID())
	return ewkbhex.Encode(polygon, ewkbhex.NDR)
}

// Scan implements the sql.Scanner interface.
func (ring *Ring) Scan(src interface{}) error {
	if src == nil {
		*ring = Ring{}
		return nil
	}

	switch src := src.(type) {
	case []uint8:
		g, err := ewkbhex.Decode(string(src))
		if err != nil {
			return err
		}
		geom, ok := g.(*geom.Polygon)
		if !ok {
			return fmt.Errorf("ring.Scan: data is not a polygon")
		}
		ring.LinearRing = *geom.LinearRing(0)
		return nil
	}

	return fmt.Errorf("cannot convert %T to Ring", src)
}

// NewGeographicRingFromExtent creates a ring in geographic coordinates that covers the geometric extent in planar crs
func NewGeographicRingFromExtent(pixToCrs *affine.Affine, width, height int, crs *godal.SpatialRef) (GeographicRing, error) {
	ring, err := NewRingFromExtent(pixToCrs, width, height, 0).cloneTo4326(crs, true)
	if err != nil {
		return GeographicRing{}, fmt.Errorf("NewGeographicRingFromExtent.%w", err)
	}
	return GeographicRing{*ring}, nil
}

// NewRingFromExtent returns the ring corresponding to the extent
func NewRingFromExtent(pixToCrs *affine.Affine, width, height, srid int) Ring {
	return NewRingFlat(srid, NewPolygonFromExtent(pixToCrs, width, height).FlatCoords())
}

// NewPolygonFromExtent returns the polygon corresponding to the extent
func NewPolygonFromExtent(pixToCrs *affine.Affine, width, height int) *geom.Polygon {
	xMin, yMin := pixToCrs.Transform(0, 0)
	xMax, yMax := pixToCrs.Transform(float64(width), float64(height))
	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}
	bounds := geom.NewBounds(geom.XY)
	bounds.SetCoords([]float64{xMin, yMin}, []float64{xMax, yMax})
	return bounds.Polygon()
}

// cloneTo4326 creates a multipolygon in 4326 coordinates that covers the shape in planar crs
// geodetic: output either geographicCoordinates (edges follow geodetic lines) or geometricCoordinates
func (s Shape) cloneTo4326(crs *godal.SpatialRef, geodetic bool) (*Shape, error) {
	g := geom.NewMultiPolygon(geom.XY)

	for i := 0; i < s.NumPolygons(); i++ {
		p := s.Polygon(i)
		gp := geom.NewPolygon(geom.XY)
		for j := 0; j < p.NumLinearRings(); j++ {
			r, err := Ring{*p.LinearRing(j)}.cloneTo4326(crs, geodetic)
			if err != nil {
				return nil, fmt.Errorf("cloneTo4326.%w", err)
			}
			gp.Push(&r.LinearRing)
		}
		g.Push(gp)
	}
	g.SetSRID(4326)

	return &Shape{*g}, nil
}

func relativeAccuracy(project *godal.Transform, x, y, lon, lat []float64) []float64 {
	// Create the midpoints
	n := len(x) - 1
	lonm, latm, acc := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		lonm[i], latm[i] = (x[i]+x[i+1])/2, (y[i]+y[i+1])/2
	}
	// Transform them to lon/lat
	project.TransformEx(lonm, latm, make([]float64, len(lon)), nil)
	// Estimate accuracy as a percentage of the length of each edge
	// MidPoint are used to be sure that the geodetic line is the right one (eg when points do not belong to the same hemisphere)
	for i := 0; i < n; i++ {
		acc[i] = (lonLatDistance(lon[i], lat[i], lonm[i], latm[i]) + lonLatDistance(lon[i+1], lat[i+1], lonm[i], latm[i])) * accuracyPc
	}
	return acc
}

// cloneTo4326 creates a ring in 4326 coordinates that covers the ring in planar crs
// geodetic: output either geographicCoordinates (edges follow geodetic lines) or geometricCoordinates
func (r Ring) cloneTo4326(crs *godal.SpatialRef, geodetic bool) (*Ring, error) {
	// Create the 4326 projection
	crsToLonLat, err := CreateLonLatProj(crs, true)
	if err != nil {
		return nil, fmt.Errorf("cloneTo4326.%w", err)
	}
	defer crsToLonLat.Close()
	projCrsToLonLat := projFromTransform(crsToLonLat)

	// Project the shape in lon/lat
	lon, lat := FlatCoordToXY(r.FlatCoords())
	crsToLonLat.TransformEx(lon, lat, make([]float64, len(lon)), nil)

	// Estimate accuracy
	x, y := FlatCoordToXY(r.FlatCoords())
	accuracyInMeter := relativeAccuracy(crsToLonLat, x, y, lon, lat)

	// Densify projection
	pts := make([]float64, 0, len(lon))
	for i := 0; i < len(accuracyInMeter); i++ {
		pts = append(pts, lon[i], lat[i])
		pts = append(pts, densifyEdge(&projCrsToLonLat, geodetic,
			x[i], y[i], x[i+1], y[i+1], lon[i], lat[i], lon[i+1], lat[i+1], accuracyInMeter[i], densifyMaxRecursion)...)
	}
	pts = append(pts, lon[0], lat[0])

	// Return a geometric shape
	p := geom.NewLinearRingFlat(geom.XY, pts)
	p.SetSRID(4326)

	return &Ring{*p}, nil
}

// lonLatDistance returns the approximate distances in meter between two lon/lat points
// TODO use Proj to compute the distance between two lon/lat points ?
func lonLatDistance(lon1, lat1, lon2, lat2 float64) float64 {
	earthRadius := 6371000.
	lon1, lat1, lon2, lat2 = DegToRad*lon1, DegToRad*lat1, DegToRad*lon2, DegToRad*lat2
	t := math.Sin(lat1)*math.Sin(lat2) + math.Cos(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
	if t > 1 {
		return 0
	}
	return earthRadius * math.Acos(t)
}

func square(f float64) float64 { return f * f }

// lonLatMidPoint returns the middle point of two lon/lat points in geographic (following the geodetic line) or in geometric coordinates
// TODO use Proj to follow the true geodetic line ?
func lonLatMidPoint(lon1, lat1, lon2, lat2 float64, geodetic bool) (float64, float64) {
	if !geodetic {
		return (lon1 + lon2) / 2, (lat1 + lat2) / 2
	}
	lon1, lat1, lon2, lat2 = DegToRad*lon1, DegToRad*lat1, DegToRad*lon2, DegToRad*lat2
	dlon := lon2 - lon1
	bx := math.Cos(lat2) * math.Cos(dlon)
	by := math.Cos(lat2) * math.Sin(dlon)
	latm := math.Atan2(
		math.Sin(lat1)+math.Sin(lat2),
		math.Sqrt(square(math.Cos(lat1)+bx)+square(by)))
	lonm := lon1 + math.Atan2(by, (math.Cos(lat1)+bx))
	return RadToDeg * lonm, RadToDeg * latm
}

type projection func(float64, float64) (float64, float64)

func projFromTransform(t *godal.Transform) projection {
	return func(x, y float64) (float64, float64) {
		tx, ty := []float64{x}, []float64{y}
		t.TransformEx(tx, ty, []float64{0}, nil)
		return tx[0], ty[0]
	}
}

const (
	accuracyPc          = 0.01
	densifyMaxRecursion = 5
)

// densifyEdge returns an array of flat lon/lat points so that the difference between
// the segment ([x1, y1], [x2, y2]) and the polyline (lon1, lat1], []returnedValue, lon2, lat2]) is lower than accuracy
func densifyEdge(projToLonLat *projection, geodetic bool, x1, y1, x2, y2, lon1, lat1, lon2, lat2, accuracy float64, recursion int) []float64 {
	// Compute middle points
	xm, ym := (x1+x2)/2, (y1+y2)/2
	lonm, latm := (*projToLonLat)(xm, ym)
	lonm2, latm2 := lonLatMidPoint(lon1, lat1, lon2, lat2, geodetic)

	// Check whether the difference is lower than accuracy
	distance := lonLatDistance(lonm, latm, lonm2, latm2)
	if distance <= accuracy {
		return []float64{}
	}

	if recursion == 0 {
		fmt.Printf("Max number of recursions reached : [%f, %f, %f, %f]->[%f, %f]~[%f, %f, %f, %f]->[%f, %f] %f>%f\n",
			x1, y1, x2, y2, lonm, latm, lon1, lat1, lon2, lat2, lonm2, latm2, distance, accuracy)
		return []float64{lonm, latm}
	}

	return append(append(
		densifyEdge(projToLonLat, geodetic, x1, y1, xm, ym, lon1, lat1, lonm, latm, accuracy, recursion-1),
		lonm, latm),
		densifyEdge(projToLonLat, geodetic, xm, ym, x2, y2, lonm, latm, lon2, lat2, accuracy, recursion-1)...)
}
