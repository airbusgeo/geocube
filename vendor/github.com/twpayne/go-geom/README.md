# go-geom

[![Build Status](https://travis-ci.org/twpayne/go-geom.svg?branch=master)](https://travis-ci.org/twpayne/go-geom)
[![GoDoc](https://godoc.org/github.com/twpayne/go-geom?status.svg)](https://godoc.org/github.com/twpayne/go-geom)
[![Go Report Card](https://goreportcard.com/badge/github.com/twpayne/go-geom)](https://goreportcard.com/report/github.com/twpayne/go-geom)
[![Coverage Status](https://coveralls.io/repos/github/twpayne/go-geom/badge.svg)](https://coveralls.io/github/twpayne/go-geom)

Package `geom` implements efficient geometry types for geospatial applications.

## Key features

* OpenGeo Consortium-style geometries.
* Support for 2D and 3D geometries, measures (time and/or distance), and
  unlimited extra dimensions.
* Encoding and decoding of common geometry formats (GeoJSON, KML, WKB, and
  others) including [`sql.Scanner`](https://godoc.org/database/sql#Scanner) and
  [`driver.Value`](https://godoc.org/database/sql/driver#Value) interface
  implementations for easy database integration.
* [2D](https://godoc.org/github.com/twpayne/go-geom/xy) and
  [3D](https://godoc.org/github.com/twpayne/go-geom/xyz) topology functions.
* Efficient, cache-friendly [internal representation](INTERNALS.md).
* Optional protection against malicious or malformed inputs.

## Examples

* [PostGIS, EWKB, and GeoJSON](https://github.com/twpayne/go-geom/tree/master/examples/postgis).

## Detailed features

### Geometry types

* [Point](https://godoc.org/github.com/twpayne/go-geom#Point)
* [LineString](https://godoc.org/github.com/twpayne/go-geom#LineString)
* [Polygon](https://godoc.org/github.com/twpayne/go-geom#Polygon)
* [MultiPoint](https://godoc.org/github.com/twpayne/go-geom#MultiPoint)
* [MultiLineString](https://godoc.org/github.com/twpayne/go-geom#MultiLineString)
* [MultiPolygon](https://godoc.org/github.com/twpayne/go-geom#MultiPolygon)
* [GeometryCollection](https://godoc.org/github.com/twpayne/go-geom#GeometryCollection)

### Encoding and decoding

* [GeoJSON](https://godoc.org/github.com/twpayne/go-geom/encoding/geojson)
* [IGC](https://godoc.org/github.com/twpayne/go-geom/encoding/igc)
* [KML](https://godoc.org/github.com/twpayne/go-geom/encoding/kml) (encoding only)
* [WKB](https://godoc.org/github.com/twpayne/go-geom/encoding/wkb)
* [EWKB](https://godoc.org/github.com/twpayne/go-geom/encoding/ewkb)
* [WKT](https://godoc.org/github.com/twpayne/go-geom/encoding/wkt) (encoding only)
* [WKB Hex](https://godoc.org/github.com/twpayne/go-geom/encoding/wkbhex)
* [EWKB Hex](https://godoc.org/github.com/twpayne/go-geom/encoding/ewkbhex)

### Geometry functions

* [XY](https://godoc.org/github.com/twpayne/go-geom/xy) 2D geometry functions
* [XYZ](https://godoc.org/github.com/twpayne/go-geom/xyz) 3D geometry functions

## Protection against malicious or malformed inputs

The WKB and EWKB formats encode geometry sizes, and memory is allocated for
those geometries. If the input is malicious or malformed, the memory allocation
can be very large, leading to a memory starvation denial-of-service attack
against the server. For example, a client might send a `MultiPoint` with header
indicating that it contains 2^32-1 points. This will result in the server
reading that geometry to allocate 2 × `sizeof(float64)` × (2^32-1) = 64GB of
memory to store those points. By default, malicious or malformed input
protection is disabled, but can be enabled by setting positive values for
`wkbcommon.MaxGeometryElements`.

## Related libraries

* [github.com/twpayne/go-gpx](https://github.com/twpayne/go-gpx) GPX encoding and decoding
* [github.com/twpayne/go-kml](https://github.com/twpayne/go-kml) KML encoding
* [github.com/twpayne/go-polyline](https://github.com/twpayne/go-polyline) Google Maps Polyline encoding and decoding
* [github.com/twpayne/go-vali](https://github.com/twpayne/go-vali) IGC validation

## License

BSD-2-Clause