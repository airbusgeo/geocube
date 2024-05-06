package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/godal"
	"github.com/google/uuid"
)

type Dataset struct {
	URI         string
	SubDir      string
	Bands       []int64
	DataMapping geocube.DataMapping
}

func (d Dataset) GDALURI() string {
	return geocube.GDALURI(d.URI, d.SubDir)
}

var ErrLogger = godal.ErrLogger(func(ec godal.ErrorCategory, code int, msg string) error {
	if ec <= godal.CE_Warning {
		return nil
	}
	return fmt.Errorf("GDAL %d: %s", code, msg)
})

type GdalDatasetDescriptor struct {
	WktCRS        string
	PixToCRS      *affine.Affine
	Width, Height int
	Bands         int
	Resampling    geocube.Resampling
	DataMapping   geocube.DataMapping
	ValidPixPc    int // Minimum percentage of valid pixels (or image not found is returned)
	Format        string
	Palette       *geocube.Palette
}

var (
	ErrNoCastToPerform = errors.New("no cast to perform")
	ErrUnableToCast    = errors.New("unableToCast")
)

// castDatasetOptions returns the translate options to create a new data with toDFormat and converts the ds.pixels fromRange toDFormat (using an non-linear mapping if exponent != 1)
func castDatasetOptions(fromRange geocube.Range, exponent float64, toDFormat geocube.DataFormat) []string {
	options := []string{
		"-ot", toDFormat.DType.ToGDAL().String(),
		"-scale", toS(fromRange.Min), toS(fromRange.Max), toS(toDFormat.Range.Min), toS(toDFormat.Range.Max),
		"-a_nodata", toS(toDFormat.NoData),
	}
	if exponent != 1 {
		options = append(options, "-exponent", toS(exponent))
	}

	return options
}

// CastDatasetOptions returns the translate options to cast fromDFormat toDFormat
// fromDFormat: NoData is ignored
func CastDatasetOptions(fromDFormat, toDFormat geocube.DataMapping) ([]string, error) {
	if fromDFormat.Equals(toDFormat) {
		return nil, ErrNoCastToPerform
	}

	// Reminder : ve = f(vi) = RangeExt.Min + (RangeExt.Max - RangeExt.Min) * ((vi - Range.Min)/(Range.Max - Range.Min))^Exponent
	// vinter = f(vfrom) = f(vto)
	// In some cases the formula is very simple !
	if toDFormat.Exponent == 1 {
		/*
			This is just a special case of the following
			if toDFormat.Range == toDFormat.RangeExt {
				return castDatasetOptions(fromDFormat.Range, fromDFormat.Exponent, geocube.DataFormat{
					DType:  toDFormat.DType,
					Range:  fromDFormat.RangeExt,
					NoData: toDFormat.NoData,
				}), nil
			}
		*/
		f := toDFormat.Range.Interval() / toDFormat.RangeExt.Interval()
		rangeEq := geocube.Range{Min: toDFormat.Range.Min + (fromDFormat.RangeExt.Min-toDFormat.RangeExt.Min)*f}
		rangeEq.Max = fromDFormat.RangeExt.Interval()*f + rangeEq.Min
		return castDatasetOptions(fromDFormat.Range, fromDFormat.Exponent, geocube.DataFormat{
			DType:  toDFormat.DType,
			Range:  rangeEq,
			NoData: toDFormat.NoData,
		}), nil
	}
	if fromDFormat.Exponent == 1 {
		f := fromDFormat.Range.Interval() / fromDFormat.RangeExt.Interval()
		rangeEq := geocube.Range{Min: fromDFormat.Range.Min + (toDFormat.RangeExt.Min-fromDFormat.RangeExt.Min)*f}
		rangeEq.Max = toDFormat.RangeExt.Interval()*f + rangeEq.Min
		return castDatasetOptions(rangeEq, 1/toDFormat.Exponent, toDFormat.DataFormat), nil
	}

	if fromDFormat.Exponent == toDFormat.Exponent {
		if fromDFormat.RangeExt.Min == toDFormat.RangeExt.Min {
			f := fromDFormat.RangeExt.Interval() / toDFormat.RangeExt.Interval()
			rangeEq := geocube.Range{
				Min: toDFormat.Range.Min,
				Max: toDFormat.Range.Interval()*math.Pow(f, 1/toDFormat.Exponent) + toDFormat.Range.Min,
			}
			return castDatasetOptions(fromDFormat.Range, 1, geocube.DataFormat{
				DType:  toDFormat.DType,
				Range:  rangeEq,
				NoData: toDFormat.NoData,
			}), nil
		}
	}

	return nil, fmt.Errorf(" Unable to cast %v to %v %w", fromDFormat, toDFormat, ErrUnableToCast)
}

func ExtractBandsOption(ds *godal.Dataset, bands []int64) []string {
	if bands == nil || (isASuite(bands) && len(ds.Bands()) == len(bands)) {
		return nil
	}
	var options []string
	for _, b := range bands {
		options = append(options, "-b", strconv.Itoa(int(b)))

	}
	return options
}

// CastDataset creates a new dataset and cast <bands> fromDFormat toDFormat
// The caller is responsible to close the dataset
// fromDFormat: NoData is ignored
// bands: if not nil, extracts the bands
// dstDS [optional] If empty, the dataset is stored in memory
func CastDataset(ctx context.Context, ds *godal.Dataset, bands []int64, fromDFormat, toDFormat geocube.DataMapping, dstDS string) (*godal.Dataset, error) {
	options, err := CastDatasetOptions(fromDFormat, toDFormat)
	if err != nil {
		return nil, err
	}
	options = append(options, ExtractBandsOption(ds, bands)...)

	if dstDS == "" {
		options = append(options, "-of", "MEM")
	}
	outDs, err := ds.Translate(dstDS, options)
	if err != nil {
		return nil, fmt.Errorf("castDataset.Translate: %w", err)
	}

	return outDs, nil
}

func closeDatasets(datasets *[]*godal.Dataset) {
	for _, ds := range *datasets {
		ds.Close()
	}
}

// MergeDatasets merge the given datasets into one in the format defined by outDesc
// The caller is responsible to close the output dataset
func MergeDatasets(ctx context.Context, datasets []*Dataset, outDesc *GdalDatasetDescriptor) (*godal.Dataset, error) {

	if len(datasets) == 0 {
		return nil, fmt.Errorf("mergeDatasets: no dataset to merge")
	}

	// Group datasets that share the same DataMapping
	groupedDatasets := [][]*Dataset{}
	for _, dataset := range datasets {
		found := false
		for i, groupedDs := range groupedDatasets {
			if dataset.DataMapping.Equals(groupedDs[0].DataMapping) {
				groupedDatasets[i] = append(groupedDatasets[i], dataset)
				found = true
				break
			}
		}
		if !found {
			groupedDatasets = append(groupedDatasets, []*Dataset{dataset})
		}
	}

	var err error
	var mergedDatasets []*godal.Dataset
	defer closeDatasets(&mergedDatasets)

	for _, groupedDs := range groupedDatasets {
		// Merge Datasets that share the same DataMapping
		commonDMapping := groupedDs[0].DataMapping
		mergedDs, err := warpDatasets(groupedDs, outDesc.WktCRS, outDesc.PixToCRS, float64(outDesc.Width), float64(outDesc.Height), outDesc.Resampling, commonDMapping.DataFormat)
		if err != nil {
			return nil, fmt.Errorf("mergeDatasets: %w", err)
		}

		// Convert dataset to outDesc.DataFormat
		if !commonDMapping.Equals(outDesc.DataMapping) {
			tmpDS := mergedDs
			mergedDs, err = CastDataset(ctx, tmpDS, nil, commonDMapping, outDesc.DataMapping, "")
			tmpDS.Close()
			if err != nil {
				return nil, fmt.Errorf("mergeDatasets: %w", err)
			}
		}
		mergedDatasets = append(mergedDatasets, mergedDs)
	}

	// Merge all the datasets together
	var mergedDs *godal.Dataset
	if len(mergedDatasets) == 1 {
		mergedDs = mergedDatasets[0]
		mergedDatasets = nil // Prevent "defer" for closing mergedDs
	} else if mergedDs, err = mosaicDatasets(mergedDatasets, outDesc.PixToCRS.Rx(), outDesc.PixToCRS.Ry()); err != nil {
		return nil, fmt.Errorf("mergeDatasets.%w", err)
	}

	// Test whether image has enough valid pixels
	if outDesc.ValidPixPc >= 0 {
		if ok, err := isValid(&mergedDs.Bands()[0], (outDesc.Width*outDesc.Height*outDesc.ValidPixPc)/100); err != nil || !ok {
			mergedDs.Close()
			if err != nil {
				return nil, fmt.Errorf("mergeDatasets.%w", err)
			}
			return nil, geocube.NewEntityNotFound("", "", "", "Not enough valid pixels (skipped)")
		}
	}

	return mergedDs, nil
}

// mosaicDatasets calls godal.Warp to merge all the datasets into one without reprojection
// The caller is responsible to close the output dataset
func mosaicDatasets(datasets []*godal.Dataset, rx, ry float64) (*godal.Dataset, error) {
	outDs, err := godal.Warp("", datasets, []string{"-tr", toS(rx), toS(ry)}, godal.Memory, ErrLogger)
	if err != nil {
		if outDs != nil {
			outDs.Close()
		}
		return nil, fmt.Errorf("failed to mosaic dataset: %w", err)
	}

	return outDs, nil
}

// isASuite return true if s = [1, 2, 3, ..., N]
func isASuite(s []int64) bool {
	for i, si := range s {
		if int64(i)+1 != si {
			return false
		}
	}
	return true
}

func warpDatasetOptions(wktCRS string, transform *affine.Affine, width, height float64, resampling geocube.Resampling, commonDFormat geocube.DataFormat) []string {

	options := []string{
		"-t_srs", wktCRS,
		"-ts", toS(width), toS(height),
		"-ovr", "AUTO", //TODO user-defined ?
		"-wo", "INIT_DEST=" + toS(commonDFormat.NoData),
		"-wm", "500",
		"-ot", commonDFormat.DType.ToGDAL().String(),
		"-r", resampling.String(),
		"-srcnodata", toS(commonDFormat.NoData),
		"-nomd",
		"-multi",
	}

	if commonDFormat.NoDataDefined() {
		options = append(options, "-dstnodata", toS(commonDFormat.NoData))
	} else {
		options = append(options, "-dstnodata", "None")
	}

	if transform != nil {
		xMin, yMax := transform.Transform(0, 0)
		xMax, yMin := transform.Transform(width, height)
		options = append(options, "-te", toS(xMin), toS(yMin), toS(xMax), toS(yMax))
	}
	return options
}

func creationOptions(creationParams map[string]string, overview bool) []string {
	var options []string
	for k, v := range creationParams {
		if overview == strings.HasSuffix(k, "_OVERVIEW") {
			options = append(options, "-co", k+"="+v)
		}
	}
	return options
}

func gtiffOptions(blockSizeX, blockSizeY int, creationParams map[string]string, bigtiff bool) []string {
	options := []string{
		"-co", "TILED=YES",
		"-co", "SPARSE_OK=TRUE",
	}
	if blockSizeX != 0 {
		options = append(options, "-co", fmt.Sprintf("BLOCKXSIZE=%d", blockSizeX))
	}
	if blockSizeY != 0 {
		options = append(options, "-co", fmt.Sprintf("BLOCKYSIZE=%d", blockSizeY))
	}
	if bigtiff {
		options = append(options, "-co", "BIGTIFF=YES")
	}

	return append(options, creationOptions(creationParams, false)...)
}

// warpDatasets calls godal.Warp on datasets, performing a reprojection
// The caller is responsible to close the output dataset
func warpDatasets(datasets []*Dataset, wktCRS string, transform *affine.Affine, width, height float64, resampling geocube.Resampling, commonDFormat geocube.DataFormat) (*godal.Dataset, error) {

	gdatasets := make([]*godal.Dataset, len(datasets))
	for i, dataset := range datasets {
		var err error
		uri := dataset.GDALURI()
		if gdatasets[i], err = godal.Open(uri, ErrLogger); err != nil {
			return nil, fmt.Errorf("while opening %s: %w", uri, err)
		}
		defer gdatasets[i].Close()
		// Need to extract bands using a vrt
		if !isASuite(dataset.Bands) || len(gdatasets[i].Bands()) > len(dataset.Bands) {
			options := []string{}
			for _, b := range dataset.Bands {
				options = append(options, "-b", strconv.Itoa(int(b)))
			}
			virtualname := "/vsimem/" + uuid.New().String() + ".vrt"
			if gdatasets[i], err = gdatasets[i].Translate(virtualname, options); err != nil {
				return nil, fmt.Errorf("while extracting bands of %s: %w", uri, err)
			}
			defer func(ds *godal.Dataset) {
				ds.Close()
				godal.VSIUnlink(virtualname)
			}(gdatasets[i])
		}
	}
	options := warpDatasetOptions(wktCRS, transform, width, height, resampling, commonDFormat)
	outDs, err := godal.Warp("", gdatasets, options, godal.Memory, ErrLogger)
	if err != nil {
		if outDs != nil {
			outDs.Close()
		}
		return nil, fmt.Errorf("failed to warp dataset: %w", err)
	}

	return outDs, nil
}

// WarpedExtent calls godal.WarpVRT on datasets, performing a reprojection and returning the extent
func WarpedExtent(ctx context.Context, datasets []*Dataset, wktCRS string, resx, resy float64) ([4]float64, error) {
	gdatasets := make([]*godal.Dataset, len(datasets))
	for i, dataset := range datasets {
		var err error
		uri := dataset.GDALURI()
		if gdatasets[i], err = godal.Open(uri, ErrLogger); err != nil {
			return [4]float64{}, fmt.Errorf("while opening %s: %w", uri, err)
		}
		defer gdatasets[i].Close()
	}

	options := []string{
		"-t_srs", wktCRS,
		"-tr", toS(resx), toS(resy),
	}

	virtualname := "/vsimem/" + uuid.New().String() + ".vrt"
	ds, err := godal.Warp(virtualname, gdatasets, options, godal.VRT, ErrLogger)
	if err != nil {
		return [4]float64{}, fmt.Errorf("failed to warp dataset: %w", err)
	}

	defer func() {
		ds.Close()
		godal.VSIUnlink(virtualname)
	}()

	return ds.Bounds()
}

func isValid(band *godal.Band, validPix int) (bool, error) {
	nodata, ok := band.NoData()
	if !ok {
		return true, nil
	}
	image, err := geocube.NewBitmapFromBand(band)
	if err != nil {
		return false, fmt.Errorf("countValidPix: %w", err)
	}
	return image.IsValid(nodata, validPix), nil
}

func toS(f float64) string {
	return utils.F64ToS(f)
}

// colorTableFromPalette creates a gdal.ColorTable from a palette
// The results must be Detroy() by the caller
func colorTableFromPalette(palette *geocube.Palette) (*godal.ColorTable, error) {
	if palette == nil {
		return nil, nil
	} else {
		return nil, fmt.Errorf("palette not supported yet")
	}
	/*
		colorTable := &godal.ColorTable{PaletteInterp: godal.PaletteInterp(godal.RGBPalette)}
		pts := make([][4]int16, len(palette.Points))
		for i, pt := range palette.Points {
			pts[i] = [4]int16{int16(pt.R), int16(pt.G), int16(pt.B), int16(pt.A)}
		}

		// Create ColorTable
		//colorTable.CreateColorRamp(0, 254, pts[0], pts[len(pts)-1])
		for i := 1; i < len(pts)-1; i++ {
			colorTable.Entries[int(palette.Points[i].Val*254)] = pts[i]
		}

		return colorTable, nil*/
}

// DatasetToPngAsBytes translates the dataset to a png and returns the byte representation
// interpolateColor is true if dataset pixel value can be interpolated
func DatasetToPngAsBytes(ctx context.Context, ds *godal.Dataset, fromDFormat geocube.DataMapping, palette *geocube.Palette, interpolateColor bool) ([]byte, error) {
	var palette256 color.Palette
	var virtualname string
	toDformat := fromDFormat

	if !interpolateColor {
		if fromDFormat.Range.Min < 0 || fromDFormat.Range.Max > 255 || fromDFormat.NoData < 0 || fromDFormat.NoData > 255 {
			return nil, fmt.Errorf("cannot create a png, because the color interpolation is forbidden")
		}
		if palette != nil {
			palette256 = palette.PaletteN(256)
		}
	} else {
		toDformat.DataFormat = geocube.DataFormat{
			DType:  geocube.DTypeUINT8,
			NoData: 255,
			Range:  geocube.Range{Min: 0, Max: 254},
		}
		toDformat.Exponent = 1

		if palette != nil {
			palette256 = palette.PaletteN(255)
			palette256 = append(palette256, color.RGBA{})
		}
	}

	if palette256 == nil { // To cast non-paletted to png
		virtualname = "/vsimem/" + uuid.New().String() + ".png"
	}

	// Cast to PNG
	pngDs, err := CastDataset(ctx, ds, nil, fromDFormat, toDformat, virtualname)
	if err != nil {
		return nil, fmt.Errorf("DatasetToPngAsBytes.%w", err)
	}
	defer func() {
		pngDs.Close()
		godal.VSIUnlink(virtualname)
	}()

	// Apply palette
	if palette256 != nil {
		bitmap, err := geocube.NewBitmapFromDataset(pngDs)
		if err != nil {
			return nil, fmt.Errorf("DatasetToPngAsBytes.%w", err)
		}
		paletted := image.NewPaletted(bitmap.Rect, palette256)
		paletted.Pix = bitmap.Bytes
		b := bytes.Buffer{}
		if err = png.Encode(&b, paletted); err != nil {
			return nil, fmt.Errorf("DatasetToPngAsBytes.PngEncode: %w", err)
		}
		return b.Bytes(), nil
	}

	// Returns byte representation of the PNG file
	vsiFile, err := godal.VSIOpen(virtualname)
	if err != nil {
		return nil, fmt.Errorf("DatasetToPngAsBytes.%w", err)
	}
	defer vsiFile.Close()

	return io.ReadAll(vsiFile)
}

// DatasetToTiffAsBytes translates the dataset to a tiff and returns the byte representation
func DatasetToTiffAsBytes(ds *godal.Dataset, fromDFormat geocube.DataMapping, tags map[string]string, palette *geocube.Palette) ([]byte, error) {
	// Todo fromDFormat is not taken into account

	// Prepare options
	var options []string
	for k, t := range tags {
		options = append(options, "-mo", fmt.Sprintf("%s=%s", k, t))
	}

	// Translate to Tiff
	virtualname := "/vsimem/" + uuid.New().String() + ".tif"
	tifDs, err := ds.Translate(virtualname, options)
	if err != nil {
		return nil, fmt.Errorf("datasetToTiff.Translate: %w", err)
	}
	defer func() {
		tifDs.Close()
		godal.VSIUnlink(virtualname)
	}()

	// Apply palette
	if palette != nil {
		tifDs.Bands()[0].SetColorInterp(godal.CIPalette)
		c, err := colorTableFromPalette(palette)
		if err != nil {
			return nil, fmt.Errorf("colorTableFromPalette: %w", err)
		}
		tifDs.Bands()[0].SetColorTable(*c)
	}

	// Returns byte representation of the TIFF file
	vsiFile, err := godal.VSIOpen(virtualname)
	if err != nil {
		return nil, fmt.Errorf("datasetToTiff.%w", err)
	}
	defer vsiFile.Close()
	return io.ReadAll(vsiFile)
}
