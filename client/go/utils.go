package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	pb "github.com/airbusgeo/geocube/client/go/pb"
	geojson "github.com/paulmach/go.geojson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AOIFromFile extracts the geometry from the given file.
// For Geojson File : it merges all the polygons and multipolygons into one multipolygon
// If provided, the CRS is ignored
func AOIFromFile(path string) (AOI, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return AOI{}, fmt.Errorf("unable to read file %s: %s", path, err)
	}

	switch filepath.Ext(path) {
	case ".json", ".geojson":
		file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))
		var geoj struct {
			Type     string             `json:"type"`
			Features []*geojson.Feature `json:"features"`
			Feature  *geojson.Feature   `json:"feature"`
			Geometry *geojson.Geometry  `json:"geometry"`
		}
		if err := json.Unmarshal(file, &geoj); err != nil {
			return AOI{}, fmt.Errorf("unable to unmarshal geojson file %s: %s", path, err)
		}

		switch strings.ToLower(geoj.Type) {
		case "featurecollection":
		case "feature":
			geoj.Features = append(geoj.Features, geoj.Feature)
		case "geometry":
			geoj.Features = append(geoj.Features, &geojson.Feature{Geometry: geoj.Geometry})
		default:
			return AOI{}, fmt.Errorf("wrong type for geojson file %s: %s", path, geoj.Type)
		}

		var aoi AOI
		for _, f := range geoj.Features {
			aoip, err := AOIFromGeometry(*f.Geometry)
			if err != nil {
				return AOI{}, fmt.Errorf("AOIFromGeojson.%w", err)
			}
			aoi = append(aoi, aoip...)
		}
		return aoi, nil

	default:
		return AOI{}, fmt.Errorf("unsupported extension: " + filepath.Ext(path))
	}
}

// AOIFromGeometry converts a geometry to an AOI
func AOIFromGeometry(geometry geojson.Geometry) (AOI, error) {
	var mp [][][][]float64
	switch geometry.Type {
	case geojson.GeometryMultiPolygon:
		mp = geometry.MultiPolygon
	case geojson.GeometryPolygon:
		mp = append(mp, geometry.Polygon)
	case geojson.GeometryMultiLineString:
		mp = append(mp, geometry.MultiLineString)
	case geojson.GeometryLineString:
		mp = append(mp, [][][]float64{geometry.LineString})
	default:
		return AOI{}, errors.New("unsupported geometry")
	}
	aoi := make([][][][2]float64, len(mp))
	for i, p := range mp {
		aoi[i] = make([][][2]float64, len(p))
		for j, l := range p {
			aoi[i][j] = make([][2]float64, len(l))
			for k, pt := range l {
				aoi[i][j][k] = [2]float64{pt[0], pt[1]}
			}
		}
	}
	return AOI(aoi), nil
}

// AOIFromMultiPolygonArray converts a multipolygon as an array to an AOI
func AOIFromMultiPolygonArray(mp [][][][2]float64) AOI {
	return AOI(mp)
}

// GeometryFromAOI converts an AOI to a geometry
func GeometryFromAOI(aoi AOI) geojson.Geometry {
	aoip := make([][][][]float64, len(aoi))
	for i, p := range aoi {
		aoip[i] = make([][][]float64, len(p))
		for j, l := range p {
			aoip[i][j] = make([][]float64, len(l))
			for k, pt := range l {
				aoip[i][j][k] = make([]float64, 2)
				copy(aoip[i][j][k], pt[0:2])
			}
		}
	}

	return geojson.Geometry{
		Type:         geojson.GeometryMultiPolygon,
		MultiPolygon: aoip,
	}
}

// pbFromAOI converts aoi to protobuf aoi
func pbFromAOI(aoi AOI) *pb.AOI {
	pbaoi := pb.AOI{Polygons: make([]*pb.Polygon, len(aoi))}
	for i, p := range aoi {
		pbaoi.Polygons[i] = &pb.Polygon{Linearrings: make([]*pb.LinearRing, len(p))}
		for j, l := range p {
			pbaoi.Polygons[i].Linearrings[j] = &pb.LinearRing{Points: make([]*pb.Coord, len(l))}
			for k, pt := range l {
				pbaoi.Polygons[i].Linearrings[j].Points[k] = &pb.Coord{Lon: float32(pt[0]), Lat: float32(pt[1])}
			}
		}
	}
	return &pbaoi
}

func aoiFromPb(pbaoi *pb.AOI) *AOI {
	if pbaoi == nil {
		return nil
	}
	aoi := make(AOI, len(pbaoi.Polygons))
	for i, p := range pbaoi.Polygons {
		aoi[i] = make([][][2]float64, len(p.Linearrings))
		for j, l := range p.Linearrings {
			aoi[i][j] = make([][2]float64, len(l.Points))
			for k, pt := range l.Points {
				aoi[i][j][k] = [2]float64{float64(pt.Lon), float64(pt.Lat)}
			}
		}
	}
	return &aoi
}

// Code is used to retrieves the status code of the error
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	for ; err != nil; err = errors.Unwrap(err) {
		if c := status.Code(err); c != codes.Unknown {
			return c
		}
	}
	return codes.Unknown
}

type tmperr struct{ error }

func (t tmperr) Temporary() bool { return true }
func (t *tmperr) Unwrap() error  { return t.error }

func grpcError(err error) error {
	if err == nil {
		return nil
	}

	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return tmperr{err}
	case codes.Aborted, codes.Canceled, codes.PermissionDenied: // Temporary ?
	case codes.AlreadyExists, codes.NotFound, codes.InvalidArgument, codes.FailedPrecondition,
		codes.Unimplemented, codes.Unauthenticated, codes.OutOfRange, codes.Internal, codes.DataLoss: //Permanent
	case codes.Unknown: // ???
	}
	return err
}
