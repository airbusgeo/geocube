[![Go Reference](https://pkg.go.dev/badge/github.com/airbusgeo/osio.svg)](https://pkg.go.dev/github.com/airbusgeo/osio)
[![License](https://img.shields.io/github/license/airbusgeo/osio.svg)](https://github.com/airbusgeo/osio/blob/main/LICENSE)
[![Build Status](https://github.com/airbusgeo/osio/workflows/build/badge.svg?branch=main&event=push)](https://github.com/airbusgeo/osio/actions?query=workflow%3Abuild+event%3Apush+branch%3Amain)
[![Coverage Status](https://coveralls.io/repos/github/airbusgeo/osio/badge.svg?branch=main)](https://coveralls.io/github/airbusgeo/osio?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/airbusgeo/osio)](https://goreportcard.com/report/github.com/airbusgeo/osio)


Osio is an object storage wrapper to expose a posix-like read-only interface to objects stored in a bucket.
It can be used to pass an object reference to functions requiring an `io.ReadSeeker` or an `io.ReaderAt`
whereas object stores only expose the equivalent of `io.ReaderAt`. 

Osio is adapted in the case where you will only be accessing a small subset of the bytes
of the remote object, for example:

- extracting a subset of files from a large tar/zip
- extracting a pixel window from a [cloud optimized geotiff](https://www.cogeo.org/)

Under the hood, osio splits the remote object into blocks of fixed sizes (by default 128k), and keeps
an lru cache of the already downloaded blocks. Subsequent reads from the object will be populated by
the contents of these cached blocks. An Osio adapter is safe for concurrent usage, and mechanisms are
in place do de-duplicate reads to the source object in case of concurrent access.


Osio has support for the following handlers:
- Google Storage,
- Amazon S3,
- Plain HTTP.

## Example Usage

### Google Storage - Zip extraction
The following example shows how to extract a single file from a (large) zip archive stored on a
Google Cloud Storage bucket.

```go
import(
    "github.com/airbusgeo/osio"
    "github.com/airbusgeo/osio/gcs"
)
func ExampleGSHandle_zip() {
    ctx := context.Background()
    gcsr, err := gcs.Handle(ctx)
    /* handle error, typically if credentials could not be found, network down ,etc... */
    gcsa, _ = osio.NewAdapter(gcsr)

    file := "gs://bucket/path/to/large/archive.zip"
    obj, err := gcsa.Reader(file)
    if err != nil {
        return fmt.Errorf("open %s: %w", file, err)
    }
    zipf, err := zip.NewReader(obj, obj.Size())
    if err != nil {
        return fmt.Errorf("zip corrupted?: %w", err)
    }
    for _, f := range zipf.File {
        if f.Name == "mytargetfile.txt" {
            fr, err := f.Open()
            dstf, err := os.Create("/local/mytargetfile.txt")
            _, err = io.Copy(dstf, fr)
            fr.Close()
            err = dstf.Close()
            //fmt.Printf("extracted %s\n", f.Name)
        }
    }
}
```


### Amazon S3 - Zip extraction


```go
import(
    aws3 "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/airbusgeo/osio"
    "github.com/airbusgeo/osio/s3"
)
func WithS3Region(region string) func(opts *aws3.Options) {
	return func(opts *aws3.Options) {
		opts.Region = region
	}
}

func ExampleS3Handle_zip() {
	ctx := context.Background()

	cfg, _ := config.LoadDefaultConfig(ctx)
	s3cl := aws3.NewFromConfig(cfg, WithS3Region("eu-central-1"))
	s3r, _ := s3.Handle(ctx, osio.S3Client(s3cl), osio.S3RequestPayer())
	osr, _ := osio.NewAdapter(s3r)

	uri := "s3://sentinel-s2-l1c-zips/S2A_MSIL1C_20210630T074611_N0300_R135_T48XWN_20210630T082841.zip"
	obj, _ := osr.Reader(uri)
	zipf, _ := zip.NewReader(obj, obj.Size())

	for _, f := range zipf.File {
		fmt.Printf("%s\n", f.Name)
		break
	}

	// Output:
	// S2A_MSIL1C_20210630T074611_N0300_R135_T48XWN_20210630T082841.SAFE/MTD_MSIL1C.xml
}
```


### GDAL I/O handler

Osio is used by the [GDAL](https://gdal.org) [godal bindings](https://github.com/airbusgeo/godal) to
enable GDAL to directly access files stored on a bucket. (Note: this mechanism only really makes sense
when accessing file formats that are object-storage friendly, e.g. [cogeotiffs](https://www.cogeo.org) )

```go
ctx := context.Background()
gcsr, err := gcs.Handle(ctx)
gcs, _ = osio.NewAdapter(gcsr)
godal.RegisterVSIAdapter("gs://", gcs)
dataset,err := godal.Open("gs://bucket/path/to/cog.tif")
...
```

## Contributing and TODOs

PRs are welcome! If you want to work on any of these things, please open an issue to coordinate.

- [ ] Azure handler
