package image_test

import (
	"math"

	"github.com/airbusgeo/geocube/internal/geocube"
)

var (
	images = []string{
		"test_data/image_cast0.tif",  // Int16[-10000, 10000] -> [-1,1]
		"test_data/image_cast1.tif",  // Float32[-1, 1] -> [-1,1]
		"test_data/image_cast2.tif",  // UInt8[0, 254] -> [-1,1]
		"test_data/image_cast3.tif",  // UInt8[0, 254] -> [0,0.5]
		"test_data/image_cast4.tif",  // UInt8[0, 254] ->^2 [-1,1]
		"test_data/image_cast5.tif",  // UInt8[0, 254] ->^2 [0,0.5]
		"test_data/image_cast6.tif",  // UInt8[0, 254] -> [0,1] (no value [0.5,1])
		"test_data/image_cast7.tif",  // Int16[-10000, 10000] ->^2 [0,1]
		"test_data/image_warp0.tif",  // Float32[0, 1] -> [0,1]
		"test_data/image_warp1.tif",  // Float32[0, 1] -> [0,1]
		"test_data/image_warp2.tif",  // Float32[0, 1] -> [0,1] (nodata=-1)
		"test_data/image_warp3.tif",  // Float32[0, 1] -> [0,1]
		"test_data/image_warp43.vrt", // Float32[0, 1] -> [0,1] (2 bands: an_image+image_warp3)
		"test_data/image_warp01.vrt", // Float32[0, 1] -> [0,1] (2 bands: an_image+image_warp1)
		"test_data/image_cast8.tif",  // Int16[-10000, 10000] -> nodata=nan
		"test_data/image_warp6.tif",  // Float32[0, 1] -> UInt8[0, 254], nodata=0
	}

	imageDFormatFloat32NoData0 = geocube.DataMapping{
		DataFormat: geocube.DataFormat{
			DType:  geocube.DTypeFLOAT32,
			NoData: 0,
			Range:  geocube.Range{Min: 0, Max: 1},
		},
		RangeExt: geocube.Range{Min: 0, Max: 1},
		Exponent: 1,
	}
	imagesDFormat = []geocube.DataMapping{
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeINT16,
				NoData: -10001,
				Range:  geocube.Range{Min: -10000, Max: 10000},
			},
			RangeExt: geocube.Range{Min: -1, Max: 1},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeFLOAT32,
				NoData: math.NaN(),
				Range:  geocube.Range{Min: -1, Max: 1},
			},
			RangeExt: geocube.Range{Min: -1, Max: 1},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 255,
				Range:  geocube.Range{Min: 0, Max: 254},
			},
			RangeExt: geocube.Range{Min: -1, Max: 1},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 255,
				Range:  geocube.Range{Min: 0, Max: 254},
			},
			RangeExt: geocube.Range{Min: 0, Max: 0.5},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 255,
				Range:  geocube.Range{Min: 0, Max: 254},
			},
			RangeExt: geocube.Range{Min: -1, Max: 1},
			Exponent: 2,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 255,
				Range:  geocube.Range{Min: 0, Max: 254},
			},
			RangeExt: geocube.Range{Min: 0, Max: 0.5},
			Exponent: 2,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 255,
				Range:  geocube.Range{Min: 0, Max: 254},
			},
			RangeExt: geocube.Range{Min: 0, Max: 1},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeINT16,
				NoData: -10001,
				Range:  geocube.Range{Min: 0, Max: 10000},
			},
			RangeExt: geocube.Range{Min: 0, Max: 1},
			Exponent: 2,
		},
		imageDFormatFloat32NoData0,
		imageDFormatFloat32NoData0,
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeFLOAT32,
				NoData: -1,
				Range:  geocube.Range{Min: 0, Max: 1},
			},
			RangeExt: geocube.Range{Min: 0, Max: 1},
			Exponent: 1,
		},
		imageDFormatFloat32NoData0,
		imageDFormatFloat32NoData0,
		imageDFormatFloat32NoData0,
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeINT16,
				NoData: math.NaN(),
				Range:  geocube.Range{Min: -10000, Max: 10000},
			},
			RangeExt: geocube.Range{Min: -1, Max: 1},
			Exponent: 1,
		},
		{
			DataFormat: geocube.DataFormat{
				DType:  geocube.DTypeUINT8,
				NoData: 0,
				Range:  geocube.Range{Min: 0, Max: 255},
			},
			RangeExt: geocube.Range{Min: 0, Max: 1},
			Exponent: 1,
		},
	}
)
