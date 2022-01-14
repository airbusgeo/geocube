package geocube

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/google/uuid"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// An AOI is by definition in *planar geometry* with *CRS 4326* (lonlat).
type AOI struct {
	ID       string
	Geometry *wkb.MultiPolygon
	hash     string // of geometry computed once on demand
}

type Record struct {
	persistenceState
	ID   string
	Name URN
	Time time.Time
	Tags Metadata
	AOI  AOI
}

func hashGeometry(g geom.T) (string, error) {
	switch gt := g.(type) {
	case *geom.LinearRing:
		g = geom.NewPolygonFlat(gt.Layout(), gt.FlatCoords(), []int{len(gt.FlatCoords())})
	}

	h := sha1.New()
	sb := &strings.Builder{}
	if err := wkb.Write(sb, wkb.NDR, g); err != nil {
		return "", fmt.Errorf("hashGeometry: %w", err)
	}
	h.Write([]byte(sb.String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashGeometry computes and returns a hash version of the geometry
// The hashing is computed only once, hence the geometry must not be changed.
func (aoi *AOI) HashGeometry() (string, error) {
	var err error

	if aoi.hash == "" {
		aoi.hash, err = hashGeometry(aoi.Geometry.MultiPolygon)
	}

	return aoi.hash, err
}

// NewRecordFromProtobuf creates a new record from protobuf and validates it
// Only returns ValidationError
func NewRecordFromProtobuf(record *pb.NewRecord) (*Record, error) {
	if err := record.GetTime().CheckValid(); err != nil {
		return nil, NewValidationError("Invalid time: " + err.Error())
	}
	r := Record{
		persistenceState: persistenceStateNEW,
		ID:               uuid.New().String(),
		Name:             URN(record.GetName()),
		Time:             record.GetTime().AsTime(),
		Tags:             record.GetTags(),
		AOI:              AOI{ID: record.GetAoiId()},
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// RecordFromProtobuf creates a new record from protobuf and validates it
// Only returns ValidationError
func RecordFromProtobuf(record *pb.Record) (*Record, error) {
	if err := record.GetTime().CheckValid(); err != nil {
		return nil, NewValidationError("Invalid time: " + err.Error())
	}
	r := Record{
		persistenceState: persistenceStateCLEAN,
		ID:               record.Id,
		Name:             URN(record.GetName()),
		Time:             record.GetTime().AsTime(),
		Tags:             record.GetTags(),
		AOI:              AOI{ID: record.GetAoiId()},
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// NewAOIFromProtobuf creates an AOI from protobuf
// Only returns ValidationError
func NewAOIFromProtobuf(polygons []*pb.Polygon, canBeEmpty bool) (*AOI, error) {
	g := geom.NewMultiPolygon(geom.XY)
	for _, polygon := range polygons {
		p := geom.NewPolygon(geom.XY)
		for _, linearring := range polygon.GetLinearrings() {
			flatCoords := make([]float64, 2*len(linearring.GetPoints()))
			for i, point := range linearring.GetPoints() {
				flatCoords[2*i] = float64(point.GetLon())
				flatCoords[2*i+1] = float64(point.GetLat())
			}
			p.Push(geom.NewLinearRingFlat(geom.XY, flatCoords))
		}
		if p.NumCoords() != 0 {
			g.Push(p)
		}
	}
	g.SetSRID(4326)

	aoi := AOI{
		ID:       uuid.New().String(),
		Geometry: &wkb.MultiPolygon{MultiPolygon: g},
	}

	return &aoi, aoi.validate(canBeEmpty)
}

// NewAOIFromMultiPolygon creates an AOI from a multi polygon
// Only returns ValidationError
func NewAOIFromMultiPolygon(geomAOI geom.MultiPolygon) (*AOI, error) {
	geomAOI.SetSRID(4326)

	aoi := AOI{
		ID:       uuid.New().String(),
		Geometry: &wkb.MultiPolygon{MultiPolygon: &geomAOI},
	}

	return &aoi, aoi.validate(false)
}

// ToProtobuf convers a record to a protobuf
func (r *Record) ToProtobuf(withAOI bool) *pb.Record {
	pbrec := pb.Record{
		Id:    r.ID,
		Name:  r.Name.string(),
		Time:  timestamppb.New(r.Time),
		Tags:  r.Tags,
		AoiId: r.AOI.ID,
	}
	if withAOI {
		pbrec.Aoi = r.AOI.ToProtobuf()
	}
	return &pbrec
}

// ToProtobuf converts an aoi to the protobuf. Could be optimized using flat_coordinates
func (aoi *AOI) ToProtobuf() *pb.AOI {
	coords := aoi.Geometry.Coords()
	pbaoi := pb.AOI{Polygons: make([]*pb.Polygon, len(coords))}
	for i, polygon := range coords {
		pbpolygons := pb.Polygon{Linearrings: make([]*pb.LinearRing, len(polygon))}
		for j, linearring := range polygon {
			pblinearring := pb.LinearRing{Points: make([]*pb.Coord, len(linearring))}
			for k, point := range linearring {
				pblinearring.Points[k] = &pb.Coord{Lon: float32(point[0]), Lat: float32(point[1])}
			}
			pbpolygons.Linearrings[j] = &pblinearring
		}
		pbaoi.Polygons[i] = &pbpolygons
	}
	return &pbaoi
}

// validate returns an error if record has an invalid format
func (r *Record) validate() error {
	if _, err := uuid.Parse(r.ID); err != nil {
		return NewValidationError("Invalid uuid: " + r.ID)
	}

	if _, err := uuid.Parse(r.AOI.ID); err != nil {
		return NewValidationError("Invalid AOI uuid: " + r.AOI.ID)
	}

	if !r.Name.valid() {
		return NewValidationError("Invalid Name: " + r.Name.string())
	}
	return nil
}

// validate returns an error if aoi has an invalid format
func (aoi *AOI) validate(canBeEmpty bool) error {
	coords := aoi.Geometry.Coords()

	if len(coords) == 0 && !canBeEmpty {
		return NewValidationError("AOI must not be empty")
	}

	for _, polygon := range coords {
		for _, linearring := range polygon {
			for _, point := range linearring {
				if point[0] < -180 || point[0] > 180 || point[1] < -90 || point[1] > 90 {
					return NewValidationError("Coordinates must be geographic (lon in [-180,180], lat in [-90,90])")
				}
			}
		}
	}
	return nil
}
