package image

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/airbusgeo/cogger"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/godal"

	"github.com/google/tiff"
)

type CogGenerator interface {
	Create(dataset *godal.Dataset, oContainer geocube.ConsolidationContainer, recordId, workDir string) (string, error)
}

func NewCogGenerator() CogGenerator {
	return &cogGenerator{}
}

type cogGenerator struct{}

func (c *cogGenerator) Create(dataset *godal.Dataset, oContainer geocube.ConsolidationContainer, recordId, workDir string) (string, error) {
	options := []string{
		"-co", "TILED=YES",
		"-co", fmt.Sprintf("BLOCKXSIZE=%d", oContainer.BlockXSize),
		"-co", fmt.Sprintf("BLOCKYSIZE=%d", oContainer.BlockYSize),
		"-co", fmt.Sprintf("NUM_THREADS=%v", "ALL_CPUS"),
		"-co", "SPARSE_OK=TRUE",
	}

	if oContainer.Compression != geocube.CompressionNO {
		options = c.addCompressionOption(oContainer, options)
	}

	if oContainer.InterleaveBands {
		options = append(options, "-co", "INTERLEAVE=BAND")
	}

	isBig := (oContainer.Width * oContainer.Height) >= (10000 * 10000)
	if isBig {
		options = append(options, "-co", "BIGTIFF=YES")
	}

	cogDatasetPath := filepath.Join(workDir, fmt.Sprintf("cog_without_overviews_%s.tif", recordId))
	cogDataset, err := dataset.Translate(cogDatasetPath, options)
	if err != nil {
		return "", fmt.Errorf("failed to translate cog: %w", err)
	}

	if err = dataset.Close(); err != nil {
		return "", fmt.Errorf("failed to close inputDataset: %w", err)
	}

	if oContainer.CreateOverviews {
		if err := c.buildOverviews(cogDataset, oContainer.ResamplingAlg); err != nil {
			return "", fmt.Errorf("failed to build overviews: %w", err)
		}
	}

	for i := 0; i < cogDataset.Structure().NBands; i++ {
		band := cogDataset.Bands()[i]
		err = band.SetNoData(oContainer.DatasetFormat.NoData)
		if err != nil {
			return "", fmt.Errorf("failed to set nodata value: %w", err)
		}
	}

	if err = cogDataset.Close(); err != nil {
		return "", fmt.Errorf("failed to close tiff file: %w", err)
	}

	finalCogDatasetPath := filepath.Join(workDir, fmt.Sprintf("cog_%s.tif", recordId))
	if err = c.rewriteTiff(cogDatasetPath, finalCogDatasetPath); err != nil {
		return "", fmt.Errorf("failed to rewrite COG file: %w", err)
	}

	return finalCogDatasetPath, nil
}

func (c *cogGenerator) addCompressionOption(container geocube.ConsolidationContainer, options []string) []string {
	switch container.DatasetFormat.DType {
	case geocube.DTypeINT8, geocube.DTypeUINT8, geocube.DTypeINT16, geocube.DTypeUINT16, geocube.DTypeINT32, geocube.DTypeUINT32:
		switch container.Compression {
		case geocube.CompressionLOSSY:
			options = append(options, "-co", "COMPRESS=LERC", "-co", "MAX_Z_ERROR=0.01")
		case geocube.CompressionLOSSLESS:
			options = append(options, "-co", "COMPRESS=ZSTD", "-co", "PREDICTOR=2")
		}
	case geocube.DTypeFLOAT32, geocube.DTypeFLOAT64:
		switch container.Compression {
		case geocube.CompressionLOSSY:
			options = append(options, "-co", "COMPRESS=LERC_ZSTD", "-co", "MAX_Z_ERROR=0.01")
		case geocube.CompressionLOSSLESS:
			options = append(options, "-co", "COMPRESS=LERC_ZSTD", "-co", "MAX_Z_ERROR=0")
		}
	case geocube.DTypeCOMPLEX64:
		options = append(options, "")
	default:
		options = append(options, "")
	}

	return options
}

func (c *cogGenerator) buildOverviews(d *godal.Dataset, resampling geocube.Resampling) error {
	overviews := c.computeOverviewLevels(d.Structure().SizeX, d.Structure().SizeY)
	if err := d.BuildOverviews(godal.Resampling(resampling.ToGDAL()), godal.Levels(overviews...)); err != nil {
		return fmt.Errorf("failed to build overviews: %w", err)
	}

	return nil
}

func (c *cogGenerator) rewriteTiff(src, dest string) error {
	file, fdesc, err := c.openDatasetTiffs(src)
	if err != nil {
		return fmt.Errorf("failed to open dataset tiffs: %w", err)
	}
	defer fdesc.Close()

	finalCogFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to rewrite cog: %w", err)
	}

	defer finalCogFile.Close()

	return cogger.Rewrite(finalCogFile, file)
}

func (c *cogGenerator) openDatasetTiffs(datasetFileName string) (tiff.ReadAtReadSeeker, io.Closer, error) {
	fd, err := os.Open(datasetFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return tiff.NewReadAtReadSeeker(fd), fd, nil
}

func (c *cogGenerator) computeOverviewLevels(width, height int) []int {
	exp := 1
	ret := make([]int, 0)
	for width > 1024 && height > 1024 {
		exp *= 2
		width /= 2
		height /= 2
		ret = append(ret, exp)
	}
	return ret
}
