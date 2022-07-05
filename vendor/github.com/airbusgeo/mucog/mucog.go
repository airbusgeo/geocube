package mucog

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/google/tiff"
	_ "github.com/google/tiff/bigtiff"
)

type SubfileType uint32

const (
	SubfileTypeImage        = 0
	SubfileTypeReducedImage = 1
	SubfileTypePage         = 2
	SubfileTypeMask         = 4
)

type PlanarConfiguration uint16

const (
	PlanarConfigurationContig   = 1
	PlanarConfigurationSeparate = 2
)

type Predictor uint16

const (
	PredictorNone          = 1
	PredictorHorizontal    = 2
	PredictorFloatingPoint = 3
)

type SampleFormat uint16

const (
	SampleFormatUInt          = 1
	SampleFormatInt           = 2
	SampleFormatIEEEFP        = 3
	SampleFormatVoid          = 4
	SampleFormatComplexInt    = 5
	SampleFormatComplexIEEEFP = 6
)

type ExtraSamples uint16

const (
	ExtraSamplesUnspecified = 0
	ExtraSamplesAssocAlpha  = 1
	ExtraSamplesUnassAlpha  = 2
)

type PhotometricInterpretation uint16

const (
	PhotometricInterpretationMinIsWhite = 0
	PhotometricInterpretationMinIsBlack = 1
	PhotometricInterpretationRGB        = 2
	PhotometricInterpretationPalette    = 3
	PhotometricInterpretationMask       = 4
	PhotometricInterpretationSeparated  = 5
	PhotometricInterpretationYCbCr      = 6
	PhotometricInterpretationCIELab     = 8
	PhotometricInterpretationICCLab     = 9
	PhotometricInterpretationITULab     = 10
	PhotometricInterpretationLOGL       = 32844
	PhotometricInterpretationLOGLUV     = 32845
)

const (
	MUCOGPattern         = "L=0>T>I>P;L=1:>I>T>P" // Full resolution is temporally interlaced, overviews are geographically interlaced
	MUCOGTemporalPattern = "L>T>I>P"              // All levels are temporally interlaced
)

type IFD struct {
	//Any field added here should also be accounted for in WriteIFD and ifd.Fieldcount
	SubfileType               uint32   `tiff:"field,tag=254"`
	ImageWidth                uint64   `tiff:"field,tag=256"`
	ImageLength               uint64   `tiff:"field,tag=257"`
	BitsPerSample             []uint16 `tiff:"field,tag=258"`
	Compression               uint16   `tiff:"field,tag=259"`
	PhotometricInterpretation uint16   `tiff:"field,tag=262"`
	DocumentName              string   `tiff:"field,tag=269"`
	SamplesPerPixel           uint16   `tiff:"field,tag=277"`
	PlanarConfiguration       uint16   `tiff:"field,tag=284"`
	DateTime                  string   `tiff:"field,tag=306"`
	Predictor                 uint16   `tiff:"field,tag=317"`
	Colormap                  []uint16 `tiff:"field,tag=320"`
	TileWidth                 uint16   `tiff:"field,tag=322"`
	TileLength                uint16   `tiff:"field,tag=323"`
	OriginalTileOffsets       []uint64 `tiff:"field,tag=324"`
	NewTileOffsets64          []uint64
	NewTileOffsets32          []uint32
	TempTileByteCounts        []uint64 `tiff:"field,tag=325"`
	TileByteCounts            []uint32
	SubIFDOffsets             []uint64 `tiff:"field,tag=330"`
	ExtraSamples              []uint16 `tiff:"field,tag=338"`
	SampleFormat              []uint16 `tiff:"field,tag=339"`
	JPEGTables                []byte   `tiff:"field,tag=347"`

	ModelPixelScaleTag     []float64 `tiff:"field,tag=33550"`
	ModelTiePointTag       []float64 `tiff:"field,tag=33922"`
	ModelTransformationTag []float64 `tiff:"field,tag=34264"`
	GeoKeyDirectoryTag     []uint16  `tiff:"field,tag=34735"`
	GeoDoubleParamsTag     []float64 `tiff:"field,tag=34736"`
	GeoAsciiParamsTag      string    `tiff:"field,tag=34737"`
	GDALMetaData           string    `tiff:"field,tag=42112"`
	LERCParams             []uint32  `tiff:"field,tag=50674"`
	RPCs                   []float64 `tiff:"field,tag=50844"`

	NoData string `tiff:"field,tag=42113"`

	SubIFDs    []*IFD
	ZoomFactor float64

	ntags                  uint64
	tagsSize               uint64
	strileSize             uint64
	nplanes                uint64 //1 if PlanarConfiguration==1, SamplesPerPixel if PlanarConfiguration==2
	ntilesx, ntilesy       uint64
	minx, miny, maxx, maxy uint64
	r                      tiff.BReader
	gt                     geotransform
}

/*
func (ifd *IFD) TagCount() uint64 {
	s, _, _ := ifd.Structure()
	return s
}
func (ifd *IFD) TagsSize() uint64 {
	_, s, _ := ifd.Structure()
	return s
}
func (ifd *IFD) StrileSize() uint64 {
	_, _, s := ifd.Structure()
	return s
}
*/

func (ifd *IFD) AddOverview(ovr *IFD) {
	ovr.SubfileType |= SubfileTypeReducedImage
	ovr.ModelPixelScaleTag = nil
	ovr.ModelTiePointTag = nil
	ovr.ModelTransformationTag = nil
	ovr.GeoAsciiParamsTag = ""
	ovr.GeoDoubleParamsTag = nil
	ovr.GeoKeyDirectoryTag = nil
	ovr.GDALMetaData = ""
	ovr.RPCs = nil

	ifd.SubIFDs = append(ifd.SubIFDs, ovr)
}

func (ifd *IFD) structure(bigtiff bool) (tagCount, ifdSize, strileSize, planeCount uint64) {
	tagCount = 0
	ifdSize = 16 //8 for field count + 8 for next ifd offset
	tagSize := uint64(20)
	planeCount = 1
	if !bigtiff {
		ifdSize = 6 // 2 for field count + 4 for next ifd offset
		tagSize = 12
	}
	strileSize = uint64(0)

	if ifd.SubfileType > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if ifd.ImageWidth > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if ifd.ImageLength > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if len(ifd.BitsPerSample) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.BitsPerSample, bigtiff)
	}
	if ifd.Compression > 0 {
		tagCount++
		ifdSize += tagSize
	}

	tagCount++ /*PhotometricInterpretation*/
	ifdSize += tagSize

	if len(ifd.DocumentName) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.DocumentName, bigtiff)
	}
	if ifd.SamplesPerPixel > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if ifd.PlanarConfiguration > 0 {
		tagCount++
		ifdSize += tagSize
		if ifd.PlanarConfiguration == PlanarConfigurationSeparate {
			planeCount = uint64(ifd.SamplesPerPixel)
		}
	}
	if len(ifd.DateTime) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.DateTime, bigtiff)
	}
	if ifd.Predictor > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if len(ifd.Colormap) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.BitsPerSample, bigtiff)
	}
	if ifd.TileWidth > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if ifd.TileLength > 0 {
		tagCount++
		ifdSize += tagSize
	}
	if len(ifd.NewTileOffsets32) > 0 {
		tagCount++
		ifdSize += tagSize
		strileSize += arrayFieldSize(ifd.NewTileOffsets32, bigtiff) - tagSize
	} else if len(ifd.NewTileOffsets64) > 0 {
		tagCount++
		ifdSize += tagSize
		strileSize += arrayFieldSize(ifd.NewTileOffsets64, bigtiff) - tagSize
	}
	if len(ifd.TileByteCounts) > 0 {
		tagCount++
		ifdSize += tagSize
		strileSize += arrayFieldSize(ifd.TileByteCounts, bigtiff) - tagSize
	}
	if len(ifd.SubIFDOffsets) > 0 {
		offs := make([]uint32, len(ifd.SubIFDOffsets))
		for i := range offs {
			offs[i] = uint32(ifd.SubIFDOffsets[i])
		}
		tagCount++
		ifdSize += arrayFieldSize(offs, bigtiff)
	}
	if len(ifd.ExtraSamples) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.ExtraSamples, bigtiff)
	}
	if len(ifd.SampleFormat) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.SampleFormat, bigtiff)
	}
	if len(ifd.JPEGTables) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.JPEGTables, bigtiff)
	}
	if len(ifd.ModelPixelScaleTag) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.ModelPixelScaleTag, bigtiff)
	}
	if len(ifd.ModelTiePointTag) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.ModelTiePointTag, bigtiff)
	}
	if len(ifd.ModelTransformationTag) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.ModelTransformationTag, bigtiff)
	}
	if len(ifd.GeoKeyDirectoryTag) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.GeoKeyDirectoryTag, bigtiff)
	}
	if len(ifd.GeoDoubleParamsTag) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.GeoDoubleParamsTag, bigtiff)
	}
	if ifd.GeoAsciiParamsTag != "" {
		tagCount++
		ifdSize += arrayFieldSize(ifd.GeoAsciiParamsTag, bigtiff)
	}
	if ifd.GDALMetaData != "" {
		tagCount++
		ifdSize += arrayFieldSize(ifd.GDALMetaData, bigtiff)
	}
	if len(ifd.LERCParams) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.LERCParams, bigtiff)
	}
	if len(ifd.RPCs) > 0 {
		tagCount++
		ifdSize += arrayFieldSize(ifd.RPCs, bigtiff)
	}
	if ifd.NoData != "" {
		tagCount++
		ifdSize += arrayFieldSize(ifd.NoData, bigtiff)
	}
	return
}

func (i *IFD) getZoomFactor(ovrResX, ovrResY uint64) float64 {
	xFactor := i.ImageWidth / ovrResX
	yFactor := i.ImageLength / ovrResY
	return math.Max(float64(xFactor), float64(yFactor))
}

type TagData struct {
	bytes.Buffer
	Offset uint64
}

func (t *TagData) NextOffset() uint64 {
	return t.Offset + uint64(t.Buffer.Len())
}

type MultiCOG struct {
	enc               binary.ByteOrder
	ifds              []*IFD
	iterators         []*Iterators
	zoomFactorToLevel map[float64]int
}

func New() *MultiCOG {
	return &MultiCOG{enc: binary.LittleEndian}
}

func (cog *MultiCOG) writeHeader(w io.Writer, bigtiff bool) error {
	if bigtiff {
		buf := [16]byte{}
		if cog.enc == binary.LittleEndian {
			copy(buf[0:], []byte("II"))
		} else {
			copy(buf[0:], []byte("MM"))
		}
		cog.enc.PutUint16(buf[2:], 43)
		cog.enc.PutUint16(buf[4:], 8)
		cog.enc.PutUint16(buf[6:], 0)
		cog.enc.PutUint64(buf[8:], 16)
		_, err := w.Write(buf[:])
		return err
	} else {
		buf := [8]byte{}
		if cog.enc == binary.LittleEndian {
			copy(buf[0:], []byte("II"))
		} else {
			copy(buf[0:], []byte("MM"))
		}
		cog.enc.PutUint16(buf[2:], 42)
		cog.enc.PutUint32(buf[4:], 8)
		_, err := w.Write(buf[:])
		return err
	}
}

const (
	TByte      = 1
	TAscii     = 2
	TShort     = 3
	TLong      = 4
	TRational  = 5
	TSByte     = 6
	TUndefined = 7
	TSShort    = 8
	TSLong     = 9
	TSRational = 10
	TFloat     = 11
	TDouble    = 12
	TLong8     = 16
	TSLong8    = 17
	TIFD8      = 18
)

func (cog *MultiCOG) computeStructure(bigtiff bool) error {
	minx, maxy := math.MaxFloat64, -math.MaxFloat64
	for i, ifd := range cog.ifds {
		var err error
		ifd.gt, err = ifd.geotransform()
		if err != nil {
			return fmt.Errorf("ifd %d geotransform: %w", i, err)
		}
		ox, oy := ifd.gt.Origin()
		if ox < minx {
			minx = ox
		}
		if oy > maxy {
			maxy = oy
		}
	}
	sx, sy := cog.ifds[0].gt.Scale()
	tsx, tsy := cog.ifds[0].TileWidth, cog.ifds[0].TileLength
	if tsx != tsy {
		return fmt.Errorf("non square tile size %dx%d", tsx, tsy)
	}
	/*
		if math.Abs(math.Abs(sx)-math.Abs(sy)) > 0.0000000001 {
			return fmt.Errorf("non square pixel scale %gx%g", sx, sy)
		}
	*/

	for i, ifd := range cog.ifds {
		ifd.ntags, ifd.tagsSize, ifd.strileSize, ifd.nplanes = ifd.structure(bigtiff)
		ifd.ntilesx = (ifd.ImageWidth + uint64(ifd.TileWidth) - 1) / uint64(ifd.TileWidth)
		ifd.ntilesy = (ifd.ImageLength + uint64(ifd.TileLength) - 1) / uint64(ifd.TileLength)

		isx, isy := ifd.gt.Scale()
		xScaleDiff := math.Abs(1 - isx/sx)
		yScaleDiff := math.Abs(1 - isy/sy)
		if xScaleDiff > 0.00000001 || yScaleDiff > 0.00000001 {
			return fmt.Errorf("ifd %d incompatible scales (x: %.16f/%.16f, y: %.16f/%.16f)", i, isx, sx, isy, sy)
		}
		if ifd.TileWidth != tsx || ifd.TileLength != tsy {
			return fmt.Errorf("ifd %d incompatible tile size (sx: %d/%d, sy: %d/%d)", i,
				ifd.TileWidth, tsx, ifd.TileLength, tsy)
		}
		if ifd.nplanes != cog.ifds[0].nplanes {
			return fmt.Errorf("ifd %d incompatible number of planes (%d/%d)", i, ifd.nplanes, cog.ifds[0].nplanes)
		}
		iox, ioy := ifd.gt.Origin()

		//pixel offset from origin of first ifd
		noffx, noffy := (iox-minx)/sx, (maxy-ioy)/sy

		//check we have no more than .1 pixel grid mis-alignment
		npx, npy := math.Mod(noffx, float64(tsx)), math.Mod(noffy, float64(tsy))
		if !(npx < 0.1 || npx > (float64(tsx)-0.1)) ||
			!(npy < 0.1 || npy > (float64(tsy)-0.1)) {
			return fmt.Errorf("ifd %d invalid grid alignment %f/%f", i, npx, npy)
		}
		ifd.minx = uint64(math.Round(noffx / float64(tsx)))
		ifd.miny = uint64(math.Round(noffy / float64(tsy)))
		ifd.maxx = ifd.minx + ifd.ntilesx
		ifd.maxy = ifd.miny + ifd.ntilesy

		for _, sifd := range ifd.SubIFDs {
			sifd.ntags, sifd.tagsSize, sifd.strileSize, sifd.nplanes = sifd.structure(bigtiff)
			sifd.ntilesx = (sifd.ImageWidth + uint64(sifd.TileWidth) - 1) / uint64(sifd.TileWidth)
			sifd.ntilesy = (sifd.ImageLength + uint64(sifd.TileLength) - 1) / uint64(sifd.TileLength)
			sifd.minx, sifd.miny, sifd.maxx, sifd.maxy = 0, 0, sifd.ntilesx, sifd.ntilesy
		}
	}
	return nil
}

func (cog *MultiCOG) computeIterator(pattern string) error {
	type MinMaxBlock struct {
		Factor float64
		MinMax [4]int32
	}

	var nbPlanes int
	zoomLevel := []MinMaxBlock{{0, [4]int32{math.MaxInt32, 0, math.MaxInt32}}}
	zFactorToLevel := map[float64]int{}
	for _, ifd := range cog.ifds {
		if ifd.SubfileType == SubfileTypeImage {
			nbPlanes = int(math.Max(float64(ifd.nplanes), float64(nbPlanes)))
			currentMM := zoomLevel[0].MinMax
			zoomLevel[0].MinMax = [4]int32{
				int32(math.Min(float64(currentMM[0]), float64(ifd.minx))),
				int32(math.Max(float64(currentMM[1]), float64(ifd.maxx))),
				int32(math.Min(float64(currentMM[2]), float64(ifd.miny))),
				int32(math.Max(float64(currentMM[3]), float64(ifd.maxy))),
			}
		}
		for _, subIfd := range ifd.SubIFDs {
			subIfd.ZoomFactor = ifd.getZoomFactor(subIfd.ImageWidth, subIfd.ImageLength)
			if subIfd.SubfileType == SubfileTypeReducedImage {
				currentLevel, ok := zFactorToLevel[subIfd.ZoomFactor]
				if !ok {
					currentLevel = len(zoomLevel)
					zFactorToLevel[subIfd.ZoomFactor] = currentLevel
					zoomLevel = append(zoomLevel, MinMaxBlock{subIfd.ZoomFactor, [4]int32{math.MaxInt32, 0, math.MaxInt32}})
				}
				currentMM := zoomLevel[currentLevel].MinMax
				zoomLevel[currentLevel].MinMax = [4]int32{
					int32(math.Min(float64(currentMM[0]), float64(subIfd.minx))),
					int32(math.Max(float64(currentMM[1]), float64(subIfd.maxx))),
					int32(math.Min(float64(currentMM[2]), float64(subIfd.miny))),
					int32(math.Max(float64(currentMM[3]), float64(subIfd.maxy))),
				}
			}
		}
	}

	// Sort zoomLevel by Factor
	sort.Slice(zoomLevel, func(i, j int) bool { return zoomLevel[i].Factor < zoomLevel[j].Factor })

	// Extract zMinMaxBlock
	zMinMaxBlock := make([][4]int32, len(zoomLevel))
	cog.zoomFactorToLevel = map[float64]int{}
	for i, z := range zoomLevel {
		cog.zoomFactorToLevel[z.Factor] = i
		zMinMaxBlock[i] = z.MinMax
	}

	var err error
	cog.iterators, err = InitIterators(pattern, len(cog.ifds), nbPlanes, zMinMaxBlock)
	if err != nil {
		return err
	}

	return nil
}

func (cog *MultiCOG) AppendIFD(ifd *IFD) {
	cog.ifds = append(cog.ifds, ifd)

}

func (cog *MultiCOG) computeImageryOffsets(bigtiff bool, pattern string) error {

	for _, mifd := range cog.ifds {
		if bigtiff {
			mifd.NewTileOffsets64 = make([]uint64, len(mifd.OriginalTileOffsets))
		} else {
			mifd.NewTileOffsets32 = make([]uint32, len(mifd.OriginalTileOffsets))
		}
		//mifd.NewTileOffsets = mifd.OriginalTileOffsets
		for _, sc := range mifd.SubIFDs {
			if bigtiff {
				sc.NewTileOffsets64 = make([]uint64, len(sc.OriginalTileOffsets))
			} else {
				sc.NewTileOffsets32 = make([]uint32, len(sc.OriginalTileOffsets))
			}
			//sc.NewTileOffsets = sc.OriginalTileOffsets
		}
	}
	err := cog.computeStructure(bigtiff)
	if err != nil {
		return err
	}

	if err = cog.computeIterator(pattern); err != nil {
		return err
	}

	//offset to start of image data
	dataOffset := uint64(16)
	if !bigtiff {
		dataOffset = 8
	}

	for _, mifd := range cog.ifds {
		dataOffset += mifd.strileSize + mifd.tagsSize
		for _, sc := range mifd.SubIFDs {
			dataOffset += sc.strileSize + sc.tagsSize
		}
	}

	datas := cog.dataInterlacing()
	tiles := datas.Tiles(cog.iterators)
	for tile := range tiles {
		tileidx := (tile.x+tile.y*tile.ifd.ntilesx)*tile.ifd.nplanes + tile.plane
		cnt := uint64(tile.ifd.TileByteCounts[tileidx])
		if cnt > 0 {
			if bigtiff {
				tile.ifd.NewTileOffsets64[tileidx] = dataOffset
			} else {
				if dataOffset > uint64(^uint32(0)) { //^uint32(0) is max uint32
					return fmt.Errorf("data would overflow tiff capacity, use bigtiff")
				}
				tile.ifd.NewTileOffsets32[tileidx] = uint32(dataOffset)
			}
			dataOffset += uint64(tile.ifd.TileByteCounts[tileidx])
		} else {
			if bigtiff {
				tile.ifd.NewTileOffsets64[tileidx] = 0
			} else {
				tile.ifd.NewTileOffsets32[tileidx] = 0
			}
		}
	}

	return nil
}

/** Write multiCOG to a mucog
 * Parameters "pattern" defines how to interlace the [I]mages (TopLevel IFD/dataset) the [P]lanes (bands), the [L]evel (zooms/overview/reduced image) and the [T]iles (geotiff blocks).
 *
 * Common patterns:
 * MUCOGPattern         = "L=0>T>I>P;L=1:>I>T>P" // Full resolution tiles are temporally interlaced, overview tiles are geographically interlaced
 * MUCOGTemporalPattern = "L>T>I>P"              // For each level, tiles are temporally interlaced
 *
 * Advanced patterns:
 * The four levels of interlacing must be prioritized in the following way L1>L2>L3>L4 where each L is in [I, P, L, T]. This order should be understood as:
 * for each L1:
 *   for each L2:
 *     for each L3:
 *       for each L4:
 *         addBlock(L1, L2, L3, L4)
 * In other words, all L4 for a given (L1, L2, L3) will be contiguous in memory.
 * For example:
 * - To optimize the access to timeseries of all the planes (such as in MUCOG): L>T>I>P => For a given zoom level and tile, all the images will be contiguous.
 * - To optimize the access to geographical information of all the planes (such as in COG) : I>L>T>P  => For a given image, zoom level and tile, all the planes will be contiguous.
 * - To optimize the access to geographical information of one plane at a time : P>I>L>T => For a given plane, image and zoom level, all the tiles will be contiguous.
 *
 * Interlacing pattern can be specialized to only select a list or a range for each level (except Tile level).
 * - By values: L=0,2,3 will only select the value 0, 2 and 3 of the level L. For example P=0,2,3 to select the corresponding planes.
 * - By range: L=0:3 will only select the values from 0 to 3 (not included) of the level L. For example P=0:3 to select the first three planes.
 * First and last values of the range can be omitted to define 0 or last element of the level. e.g P=2: means all the planes from the second.
 * L=0 is the full resolution, L=1 is the first overview (usually: zoom factor=2), L=2 is the second overiew (usually: zoom factor=4), and so on.
 *
 * To chain interlacing patterns, use ";" separator.
 *
 * For example:
 * - Optimize access to timeseries for full resolution (L=0), but geographic for overviews (L=1:). L=0>T>I>P;L=1:>I>T>P
 * - Same example, but the planes are separated: P>L=0>T>I;P>L=1:>I>T
 * - To optimize access to geographic information of the three first planes together, but timeseries of the others: L>T>I>P=0:3;P=3:>L>I>T
 *
 * There is no validation that the pattern includes all the tiles (the others will be lost, e.g. L=0>T>I>P removes all the overviews), neither that the pattern has duplicated tiles (unpredictable behavior: e.g. L>T>I>P=0;L>T>I>P=0:2 : P=0 is duplicated).
 */
func (cog *MultiCOG) Write(out io.Writer, bigtiff bool, pattern string) error {
	for _, mifd := range cog.ifds {
		if len(mifd.SubIFDOffsets) != len(mifd.SubIFDs) {
			mifd.SubIFDOffsets = make([]uint64, len(mifd.SubIFDs))
		}
	}

	err := cog.computeImageryOffsets(bigtiff, pattern)
	if err != nil {
		return err
	}

	//compute start of strile data, and offsets to subIFDs
	//striles are placed after all ifds
	strileData := &TagData{Offset: 16}
	if !bigtiff {
		strileData.Offset = 8
	}

	for _, mifd := range cog.ifds {
		strileData.Offset += mifd.tagsSize
		for si, sc := range mifd.SubIFDs {
			mifd.SubIFDOffsets[si] = strileData.Offset
			strileData.Offset += sc.tagsSize
		}
	}

	cog.writeHeader(out, bigtiff)

	off := uint64(16)
	if !bigtiff {
		off = 8
	}
	for i, mifd := range cog.ifds {
		//compute offset of next top level ifd
		//it's the current offset, plus length of current ifd + subifds
		next := uint64(0)
		if i != len(cog.ifds)-1 {
			next = off + mifd.tagsSize
			for _, sifd := range mifd.SubIFDs {
				next += sifd.tagsSize
			}
		}
		//log.Printf("%d offsets: %v", i, mifd.NewTileOffsets)
		err := cog.writeIFD(out, bigtiff, mifd, off, strileData, next)
		if err != nil {
			return fmt.Errorf("write ifd %d: %w", i, err)
		}
		off += mifd.tagsSize
		for s, sifd := range mifd.SubIFDs {
			//log.Printf("%d/%d offsets: %v", i, s, sifd.NewTileOffsets)
			err := cog.writeIFD(out, bigtiff, sifd, off, strileData, 0)
			if err != nil {
				return fmt.Errorf("write subifd %d/%d:%w", i, s, err)
			}
			off += sifd.tagsSize
		}
	}

	//write all subifds
	_, err = out.Write(strileData.Bytes())

	datas := cog.dataInterlacing()
	tiles := datas.Tiles(cog.iterators)
	buf := &bytes.Buffer{}
	for tile := range tiles {
		buf.Reset()
		idx := (tile.x+tile.y*tile.ifd.ntilesx)*tile.ifd.nplanes + tile.plane
		if tile.ifd.TileByteCounts[idx] > 0 {
			_, err := tile.ifd.r.Seek(int64(tile.ifd.OriginalTileOffsets[idx]), io.SeekStart)
			if err != nil {
				return fmt.Errorf("seek to %d: %w", tile.ifd.OriginalTileOffsets[idx], err)
			}
			_, err = io.CopyN(out, tile.ifd.r, int64(tile.ifd.TileByteCounts[idx]))
			if err != nil {
				return fmt.Errorf("copy %d from %d: %w",
					tile.ifd.TileByteCounts[idx], tile.ifd.OriginalTileOffsets[idx], err)
			}
		}
	}

	return err
}

func (cog *MultiCOG) writeIFD(w io.Writer, bigtiff bool, ifd *IFD, offset uint64, striledata *TagData, next uint64) error {

	var err error
	// Make space for "pointer area" containing IFD entry data
	// longer than 4 bytes.
	overflow := &TagData{
		Offset: offset + 8 + 20*ifd.ntags + 8,
	}
	if !bigtiff {
		overflow.Offset = offset + 2 + 12*ifd.ntags + 4
	}

	if bigtiff {
		err = binary.Write(w, cog.enc, ifd.ntags)
	} else {
		err = binary.Write(w, cog.enc, uint16(ifd.ntags))
	}
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	if ifd.SubfileType > 0 {
		err := cog.writeField(w, bigtiff, 254, ifd.SubfileType)
		if err != nil {
			panic(err)
		}
	}
	if ifd.ImageWidth > 0 {
		err := cog.writeField(w, bigtiff, 256, uint32(ifd.ImageWidth))
		if err != nil {
			panic(err)
		}
	}
	if ifd.ImageLength > 0 {
		err := cog.writeField(w, bigtiff, 257, uint32(ifd.ImageLength))
		if err != nil {
			panic(err)
		}
	}

	if len(ifd.BitsPerSample) > 0 {
		err := cog.writeArray(w, bigtiff, 258, ifd.BitsPerSample, overflow)
		if err != nil {
			panic(err)
		}
	}

	if ifd.Compression > 0 {
		err := cog.writeField(w, bigtiff, 259, ifd.Compression)
		if err != nil {
			panic(err)
		}
	}

	err = cog.writeField(w, bigtiff, 262, ifd.PhotometricInterpretation)
	if err != nil {
		panic(err)
	}

	//DocumentName              string   `tiff:"field,tag=269"`
	if len(ifd.DocumentName) > 0 {
		err := cog.writeArray(w, bigtiff, 269, ifd.DocumentName, overflow)
		if err != nil {
			panic(err)
		}
	}

	//SamplesPerPixel           uint16   `tiff:"field,tag=277"`
	if ifd.SamplesPerPixel > 0 {
		err := cog.writeField(w, bigtiff, 277, ifd.SamplesPerPixel)
		if err != nil {
			panic(err)
		}
	}

	//PlanarConfiguration       uint16   `tiff:"field,tag=284"`
	if ifd.PlanarConfiguration > 0 {
		err := cog.writeField(w, bigtiff, 284, ifd.PlanarConfiguration)
		if err != nil {
			panic(err)
		}
	}

	//DateTime                  string   `tiff:"field,tag=306"`
	if len(ifd.DateTime) > 0 {
		err := cog.writeArray(w, bigtiff, 306, ifd.DateTime, overflow)
		if err != nil {
			panic(err)
		}
	}

	//Predictor                 uint16   `tiff:"field,tag=317"`
	if ifd.Predictor > 0 {
		err := cog.writeField(w, bigtiff, 317, ifd.Predictor)
		if err != nil {
			panic(err)
		}
	}

	//Colormap                  []uint16 `tiff:"field,tag=320"`
	if len(ifd.Colormap) > 0 {
		err := cog.writeArray(w, bigtiff, 320, ifd.Colormap, overflow)
		if err != nil {
			panic(err)
		}
	}

	//TileWidth                 uint16   `tiff:"field,tag=322"`
	if ifd.TileWidth > 0 {
		err := cog.writeField(w, bigtiff, 322, ifd.TileWidth)
		if err != nil {
			panic(err)
		}
	}

	//TileHeight                uint16   `tiff:"field,tag=323"`
	if ifd.TileLength > 0 {
		err := cog.writeField(w, bigtiff, 323, ifd.TileLength)
		if err != nil {
			panic(err)
		}
	}

	//TileOffsets               []uint64 `tiff:"field,tag=324"`
	if len(ifd.NewTileOffsets32) > 0 {
		err := cog.writeArray(w, bigtiff, 324, ifd.NewTileOffsets32, striledata)
		if err != nil {
			panic(err)
		}
	} else {
		err := cog.writeArray(w, bigtiff, 324, ifd.NewTileOffsets64, striledata)
		if err != nil {
			panic(err)
		}
	}

	//TileByteCounts            []uint32 `tiff:"field,tag=325"`
	if len(ifd.TileByteCounts) > 0 {
		err := cog.writeArray(w, bigtiff, 325, ifd.TileByteCounts, striledata)
		if err != nil {
			panic(err)
		}
	}

	//SubIFDOffsets             []uint64 `tiff:"field,tag=330"`
	if len(ifd.SubIFDOffsets) > 0 {
		offs := make([]uint32, len(ifd.SubIFDOffsets))
		for i := range offs {
			if ifd.SubIFDOffsets[i] > uint64(^uint32(0)) {
				panic("subifdoffset too big")
			}
			offs[i] = uint32(ifd.SubIFDOffsets[i])
		}
		err := cog.writeArray(w, bigtiff, 330, offs, overflow)
		if err != nil {
			panic(err)
		}
	}

	//ExtraSamples              []uint16 `tiff:"field,tag=338"`
	if len(ifd.ExtraSamples) > 0 {
		err := cog.writeArray(w, bigtiff, 338, ifd.ExtraSamples, overflow)
		if err != nil {
			panic(err)
		}
	}

	//SampleFormat              []uint16 `tiff:"field,tag=339"`
	if len(ifd.SampleFormat) > 0 {
		err := cog.writeArray(w, bigtiff, 339, ifd.SampleFormat, overflow)
		if err != nil {
			panic(err)
		}
	}

	//JPEGTables                []byte   `tiff:"field,tag=347"`
	if len(ifd.JPEGTables) > 0 {
		err := cog.writeArray(w, bigtiff, 347, ifd.JPEGTables, overflow)
		if err != nil {
			panic(err)
		}
	}

	//ModelPixelScaleTag     []float64 `tiff:"field,tag=33550"`
	if len(ifd.ModelPixelScaleTag) > 0 {
		err := cog.writeArray(w, bigtiff, 33550, ifd.ModelPixelScaleTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	//ModelTiePointTag       []float64 `tiff:"field,tag=33922"`
	if len(ifd.ModelTiePointTag) > 0 {
		err := cog.writeArray(w, bigtiff, 33922, ifd.ModelTiePointTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	//ModelTransformationTag []float64 `tiff:"field,tag=34264"`
	if len(ifd.ModelTransformationTag) > 0 {
		err := cog.writeArray(w, bigtiff, 34264, ifd.ModelTransformationTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	//GeoKeyDirectoryTag     []uint16  `tiff:"field,tag=34735"`
	if len(ifd.GeoKeyDirectoryTag) > 0 {
		err := cog.writeArray(w, bigtiff, 34735, ifd.GeoKeyDirectoryTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	//GeoDoubleParamsTag     []float64 `tiff:"field,tag=34736"`
	if len(ifd.GeoDoubleParamsTag) > 0 {
		err := cog.writeArray(w, bigtiff, 34736, ifd.GeoDoubleParamsTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	//GeoAsciiParamsTag      string    `tiff:"field,tag=34737"`
	if len(ifd.GeoAsciiParamsTag) > 0 {
		err := cog.writeArray(w, bigtiff, 34737, ifd.GeoAsciiParamsTag, overflow)
		if err != nil {
			panic(err)
		}
	}

	if ifd.GDALMetaData != "" {
		err := cog.writeArray(w, bigtiff, 42112, ifd.GDALMetaData, overflow)
		if err != nil {
			panic(err)
		}
	}
	//NoData string `tiff:"field,tag=42113"`
	if len(ifd.NoData) > 0 {
		err := cog.writeArray(w, bigtiff, 42113, ifd.NoData, overflow)
		if err != nil {
			panic(err)
		}
	}
	if len(ifd.LERCParams) > 0 {
		err := cog.writeArray(w, bigtiff, 50674, ifd.LERCParams, overflow)
		if err != nil {
			panic(err)
		}
	}
	if len(ifd.RPCs) > 0 {
		err := cog.writeArray(w, bigtiff, 50844, ifd.RPCs, overflow)
		if err != nil {
			panic(err)
		}
	}

	if bigtiff {
		err = binary.Write(w, cog.enc, next)
	} else {
		err = binary.Write(w, cog.enc, uint32(next))
	}
	if err != nil {
		return fmt.Errorf("write next: %w", err)
	}
	_, err = w.Write(overflow.Bytes())
	if err != nil {
		return fmt.Errorf("write parea: %w", err)
	}
	return nil
}

type tile struct {
	ifd   *IFD
	x, y  uint64
	plane uint64
}

func (cog *MultiCOG) dataInterlacing() datas {
	var result datas
	for _, topifd := range cog.ifds {
		data := [][]*IFD{{topifd}}
		for _, subifd := range topifd.SubIFDs {
			z := cog.zoomFactorToLevel[subifd.ZoomFactor]
			for i := len(data); i <= z; i++ {
				data = append(data, []*IFD{})
			}
			data[z] = append(data[z], subifd)
		}
		// Sort each zoom level
		for i := range data {
			sort.Slice(data[i], func(k, l int) bool {
				return data[i][k].SubfileType < data[i][l].SubfileType
			})
		}
		result = append(result, data)
	}

	return result
}

type datas [][][]*IFD

func (d datas) Tiles(iterators []*Iterators) chan tile {
	ch := make(chan tile)
	go func() {
		defer close(ch)
		for _, it := range iterators {
			indices := []*int{nil, nil, nil, nil}
			for it[0].Init(indices); it[0].Next(); {
				for it[1].Init(indices); it[1].Next(); {
					for it[2].Init(indices); it[2].Next(); {
						for it[3].Init(indices); it[3].Next(); {
							x, y := DecodePair(*indices[IDX_TILE])
							p := uint64(*indices[IDX_PLANE])
							for _, ifd := range d[*indices[IDX_IMAGE]][*indices[IDX_LEVEL]] {
								if uint64(x) >= ifd.minx && uint64(x) < ifd.maxx && uint64(y) >= ifd.miny && uint64(y) < ifd.maxy {
									ch <- tile{
										ifd:   ifd,
										x:     uint64(x),
										y:     uint64(y),
										plane: p,
									}
								}
							}
						}
					}
				}
			}
		}
	}()
	return ch
}
