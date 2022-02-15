package image

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/airbusgeo/geocube/internal/utils"

	"github.com/airbusgeo/cogger"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/godal"

	"github.com/google/tiff"
)

type CogGenerator interface {
	Create(dataset *godal.Dataset, oContainer geocube.ConsolidationContainer, recordId, workDir string) (string, error)
	Open(ctx context.Context, filePath string) (*godal.Dataset, error)
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

	cogDatasetPath := filepath.Join("/vsimem", fmt.Sprintf("cog_without_overviews_%s.tif", recordId))
	cogDataset, err := dataset.Translate(cogDatasetPath, options)
	if err != nil {
		return "", fmt.Errorf("failed to translate cog: %w", err)
	}
	defer func() {
		cogDataset.Close()
		godal.VSIUnlink(cogDatasetPath)
	}()

	if oContainer.OverviewsMinSize != geocube.NO_OVERVIEW {
		if oContainer.OverviewsMinSize == geocube.OVERVIEWS_DEFAULT_MIN_SIZE {
			oContainer.OverviewsMinSize = 256
		}
		if err := c.buildOverviews(cogDataset, oContainer.ResamplingAlg, oContainer.OverviewsMinSize); err != nil {
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

/*
	Open a cog or return an error if file is not a valid COG (see: https://github.com/rouault/cog_validator/blob/master/validate_cloud_optimized_geotiff.py)
	The caller is responible for closing the dataset
*/
func (c *cogGenerator) Open(ctx context.Context, filepath string) (*godal.Dataset, error) {
	ds, err := godal.Open(filepath, godal.Drivers("GTiff"))
	if err != nil {
		return nil, err
	}

	band := ds.Bands()[0]
	ovrCount := len(band.Overviews())
	sizeX := band.Structure().SizeX
	sizeY := band.Structure().SizeY
	blockSizeX := band.Structure().BlockSizeX
	blockSizeY := band.Structure().BlockSizeY

	if sizeX > 512 || sizeY > 512 {
		if (blockSizeX == sizeX && blockSizeX > 1024) || (blockSizeY == sizeY && blockSizeY > 1024) {
			err = utils.MergeErrors(true, err, fmt.Errorf("file is greater than 1024xHeight or Widthx1024, but is not tiled"))
		}
	}

	var ifdOffsets []int
	ifdOffset, e := strconv.Atoi(band.Metadata("IFD_OFFSET", godal.Domain("TIFF")))
	if e != nil {
		err = utils.MergeErrors(true, err, e)
	}
	ifdOffsets = append(ifdOffsets, ifdOffset)

	for i := 0; i < ovrCount; i++ {
		ovrBand := band.Overviews()[i]
		if i == 0 {
			if ovrBand.Structure().SizeX > sizeX || ovrBand.Structure().SizeY > sizeY {
				err = utils.MergeErrors(true, err, fmt.Errorf("first overview has larger dimension than main band"))
			}
		} else {
			previousOvrBand := band.Overviews()[i-1]
			if ovrBand.Structure().SizeX > previousOvrBand.Structure().SizeX || ovrBand.Structure().SizeY > previousOvrBand.Structure().SizeY {
				err = utils.MergeErrors(true, err, fmt.Errorf("overview of index %d has larger dimension than overview of index %d", i, i-1))
			}
		}

		blockSizeXBandOvr := ovrBand.Structure().BlockSizeX
		blockSizeYBandOvr := ovrBand.Structure().BlockSizeY
		if (blockSizeXBandOvr == sizeX && blockSizeXBandOvr > 1024) || (blockSizeYBandOvr == sizeY && blockSizeYBandOvr > 1024) {
			err = utils.MergeErrors(true, err, fmt.Errorf("overview of index %d is not tiled", i))
		}

		if ifdOffset, e = strconv.Atoi(ovrBand.Metadata("IFD_OFFSET", godal.Domain("TIFF"))); e != nil {
			err = utils.MergeErrors(true, err, e)
		}
		ifdOffsets = append(ifdOffsets, ifdOffset)
		if ifdOffsets[len(ifdOffsets)-1] < ifdOffsets[len(ifdOffsets)-2] {
			if i == 0 {
				err = utils.MergeErrors(true, err, fmt.Errorf("the offset of the IFD for overview of index %d is %d, whereas it should be greater than the one of the main image, which is at byte %d", i, ifdOffsets[len(ifdOffsets)-1], ifdOffsets[len(ifdOffsets)-2]))
			} else {
				err = utils.MergeErrors(true, err, fmt.Errorf("the offset of the IFD for overview of index %d is %d, whereas it should be greater than the one of index %d, which is at byte %d", i, ifdOffsets[len(ifdOffsets)-1], i-1, ifdOffsets[len(ifdOffsets)-2]))
			}
		}
	}

	blockOffset := c.getBlockOffset(band)
	dataOffsets := []int{blockOffset}
	for i := 0; i < ovrCount; i++ {
		ovrBand := band.Overviews()[i]
		blockOffset = c.getBlockOffset(ovrBand)
		dataOffsets = append(dataOffsets, blockOffset)
	}

	if dataOffsets[len(dataOffsets)-1] != 0 && dataOffsets[len(dataOffsets)-1] < ifdOffsets[len(ifdOffsets)-1] {
		if ovrCount > 0 {
			err = utils.MergeErrors(true, err, fmt.Errorf("the offset of the first block of the smallest overview should be after its IFD"))
		} else {
			err = utils.MergeErrors(true, err, fmt.Errorf("the offset of the first block of the image should be after its IFD"))
		}
	}

	if len(dataOffsets) >= 2 && dataOffsets[0] != 0 && dataOffsets[0] < dataOffsets[1] {
		err = utils.MergeErrors(true, err, fmt.Errorf("the offset of the first block of the main resolution image should be after the one of the overview of index %d", ovrCount-1))
	}

	if err != nil {
		ds.Close()
		return nil, err
	}
	return ds, nil
}

func (c *cogGenerator) getBlockOffset(band godal.Band) int {
	blockSizeX, blockSizeY := band.Structure().BlockSizeX, band.Structure().BlockSizeY
	for y := 0; y < (band.Structure().SizeY+blockSizeY-1)/blockSizeY; y++ {
		for x := 0; x < (band.Structure().SizeX+blockSizeX-1)/blockSizeX; x++ {
			blockOffset := band.Metadata(fmt.Sprintf("BLOCK_OFFSET_%d_%d", x, y), godal.Domain("TIFF"))
			if blockOffset != "" {
				i, err := strconv.Atoi(blockOffset)
				if err != nil {
					return -1
				}
				return i
			}
		}
	}
	return -1
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

func (c *cogGenerator) buildOverviews(d *godal.Dataset, resampling geocube.Resampling, overviewsMinSize int) error {
	if err := d.BuildOverviews(godal.Resampling(resampling.ToGDAL()), godal.MinSize(overviewsMinSize)); err != nil {
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
	fd, err := godal.VSIOpen(datasetFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return tiff.NewReadAtReadSeeker(fd), fd, nil
}
