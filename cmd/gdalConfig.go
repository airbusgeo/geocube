package cmd

import (
	"context"
	"flag"
	"io"
	"os"

	"github.com/airbusgeo/geocube/interface/storage/gcs"
	"github.com/airbusgeo/godal"
	"github.com/airbusgeo/osio"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"

	osioGcs "github.com/airbusgeo/osio/gcs"
	osioS3 "github.com/airbusgeo/osio/s3"
	aws3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type GDALConfig struct {
	BlockSize       string
	NumCachedBlocks int
	StorageDebug    bool
	WithGCS         bool
	WithS3          bool
	AwsRegion       string
	AwsEndpoint     string
	AwsCredentials  string
	RegisterPNG     bool
}

const (
	BlockSize       = "gdalBlockSize"
	NumCachedBlocks = "gdalNumCachedBlocks"
	WithGCS         = "with-gcs"
	WithS3          = "with-s3"
	AWSRegion       = "aws-region"
	AWSEndPoint     = "aws-endpoint"
	AwsCredentials  = "aws-shared-credentials-file"
	StorageDebug    = "gdalStorageDebug"
)

type GDALConfigFlags map[string]interface{}

func GDALConfigGetFlags() GDALConfigFlags {
	return map[string]interface{}{
		BlockSize:       flag.String(BlockSize, "1Mb", "gdal blocksize value (default 1Mb)"),
		NumCachedBlocks: flag.Int(NumCachedBlocks, 500, "gdal blockcache value (default 500)"),
		WithGCS:         flag.Bool(WithGCS, false, "configure GDAL to use gcs storage (may need authentication)"),
		WithS3:          flag.Bool(WithS3, false, "configure GDAL to use s3 storage (may need authentication)"),
		AWSRegion:       flag.String(AWSRegion, "", "define aws_region for GDAL to use s3 storage (--with-s3)"),
		AWSEndPoint:     flag.String(AWSEndPoint, "", "define aws_endpoint for GDAL to use s3 storage (--with-s3)"),
		AwsCredentials:  flag.String(AwsCredentials, "", "define aws_shared_credentials_file for GDAL to use s3 storage (--with-s3)"),
		StorageDebug:    flag.Bool(StorageDebug, false, "enable storage debug to use custom gdal storage strategy"),
	}
}

func NewGDALConfig(flags GDALConfigFlags) (*GDALConfig, error) {
	return &GDALConfig{
		BlockSize:       *flags[BlockSize].(*string),
		NumCachedBlocks: *flags[NumCachedBlocks].(*int),
		WithGCS:         *flags[WithGCS].(*bool),
		WithS3:          *flags[WithS3].(*bool),
		AwsRegion:       *flags[AWSRegion].(*string),
		AwsEndpoint:     *flags[AWSEndPoint].(*string),
		AwsCredentials:  *flags[AwsCredentials].(*string),
		StorageDebug:    *flags[StorageDebug].(*bool),
	}, nil
}

func InitGDAL(ctx context.Context, gdalConfig *GDALConfig) error {
	os.Setenv("GDAL_DISABLE_READDIR_ON_OPEN", "EMPTY_DIR")

	godal.RegisterAll()
	if gdalConfig.RegisterPNG {
		if err := godal.RegisterRaster("PNG"); err != nil {
			return err
		}
	}

	var adapter interface {
		StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error)
	}

	switch {
	case gdalConfig.WithGCS:
		var err error
		if gdalConfig.StorageDebug {
			adapter, err = gcs.NewGsStrategy(ctx)
			if err != nil {
				return err
			}
		} else {
			adapter, err = osioGcs.Handle(ctx)
			if err != nil {
				return err
			}
		}
		gcsa, err := osio.NewAdapter(adapter,
			osio.BlockSize(gdalConfig.BlockSize),
			osio.NumCachedBlocks(gdalConfig.NumCachedBlocks))
		if err != nil {
			return err
		}
		if err = godal.RegisterVSIHandler("gs://", gcsa); err != nil {
			return err
		}
	case gdalConfig.WithS3:
		resolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               gdalConfig.AwsEndpoint,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		})

		config, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithSharedCredentialsFiles([]string{gdalConfig.AwsCredentials}),
			awsConfig.WithRegion(gdalConfig.AwsRegion),
			awsConfig.WithEndpointResolver(resolver),
		)
		if err != nil {
			return err
		}

		s3Client := aws3.NewFromConfig(config)
		osioS3Handle, err := osioS3.Handle(ctx, osioS3.S3Client(s3Client))
		if err != nil {
			return err
		}

		s3Adapter, err := osio.NewAdapter(osioS3Handle,
			osio.BlockSize(gdalConfig.BlockSize),
			osio.NumCachedBlocks(gdalConfig.NumCachedBlocks))
		if err != nil {
			return err
		}

		err = godal.RegisterVSIHandler("s3://", s3Adapter)
		if err != nil {
			return err
		}

	default:
		/*if gdalConfig.StorageDebug {
			// TODO configuration with filesystem strategy
		}*/
		// Else no debug > Nothing to do
	}

	return nil
}
