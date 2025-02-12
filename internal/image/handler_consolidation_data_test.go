package image_test

import (
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils/bitmap"
	"github.com/airbusgeo/mucog"
)

var (
	ConsolidationEvent1Record = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp3.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:                `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:          [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:              256,
			Height:             256,
			Cutline:            "",
			BandsCount:         1,
			BlockXSize:         256,
			BlockYSize:         256,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      geocube.ResamplingNEAR,
			CreationParams:     map[string]string{},
			StorageClass:       geocube.StorageClassSTANDARD,
		},
	}
	ConsolidationEvent1RecordRGB = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp5.tif",
						Subdir:    "",
						Bands:     []int64{1, 2, 3},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeUINT8,
								Range:  geocube.Range{Min: 0, Max: 255},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeUINT8,
					Range:  geocube.Range{Min: 0, Max: 255},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:                `PROJCS["WGS 84 / Pseudo-Mercator",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Mercator_1SP"],PARAMETER["central_meridian",0],PARAMETER["scale_factor",1],PARAMETER["false_easting",0],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],EXTENSION["PROJ4","+proj=merc +a=6378137 +b=6378137 +lat_ts=0 +lon_0=0 +x_0=0 +y_0=0 +k=1 +units=m +nadgrids=@null +wktext +no_defs"],AUTHORITY["EPSG","3857"]]`,
			Transform:          [6]float64{994443.0, 10.0, 0.0, 7608734.0, 0.0, -10.0},
			Width:              38,
			Height:             32,
			Cutline:            "",
			BandsCount:         3,
			BlockXSize:         256,
			BlockYSize:         256,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      geocube.ResamplingNEAR,
			CreationParams:     map[string]string{"COMPRESS": "JPEG"},
			StorageClass:       geocube.StorageClassSTANDARD,
		},
	}

	ConsolidationEvent1Record2dataset = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp1.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					}, {
						URI:       "test_data/image_warp2.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: -1,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:                `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:          [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:              256,
			Height:             256,
			Cutline:            "",
			BandsCount:         1,
			BlockXSize:         256,
			BlockYSize:         256,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      geocube.ResamplingNEAR,
			CreationParams:     map[string]string{},
			StorageClass:       geocube.StorageClassSTANDARD,
		},
	}

	ConsolidationEvent2Record = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp3.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			}, {
				ID:       "recordID2",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp4.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:                `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:          [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:              256,
			Height:             256,
			Cutline:            "",
			BandsCount:         1,
			BlockXSize:         256,
			BlockYSize:         256,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      geocube.ResamplingNEAR,
			CreationParams:     map[string]string{},
			StorageClass:       geocube.StorageClassSTANDARD,
		},
	}

	ConsolidationEvent1RecordOtherDataFormat = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp3.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeFLOAT64,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:                `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:          [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:              256,
			Height:             256,
			Cutline:            "",
			BandsCount:         1,
			BlockXSize:         256,
			BlockYSize:         256,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      geocube.ResamplingNEAR,
			CreationParams:     map[string]string{},
			StorageClass:       geocube.StorageClassSTANDARD,
		},
	}

	ConsolidationEvent = &geocube.ConsolidationEvent{
		JobID:  "JobID",
		TaskID: "TaskID",
		Records: []geocube.ConsolidationRecord{
			{
				ID:       "recordID1",
				DateTime: "2020-11-02 09:44:00",
				Datasets: []geocube.ConsolidationDataset{
					{
						URI:       "test_data/image_warp3.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  bitmap.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					},
				},
			},
		},
		Container: geocube.ConsolidationContainer{
			URI: "file://test_data/mucog.tif",
			DatasetFormat: geocube.DataMapping{
				DataFormat: geocube.DataFormat{
					DType:  bitmap.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{
					Min: 0,
					Max: 1,
				},
				Exponent: 0,
			},
			CRS:                `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:          [6]float64{460943.9866, 200.19801980198008, 0, 6.2551182875e+06, 0, -200.19900497512438},
			Width:              505,
			Height:             201,
			Cutline:            "",
			BandsCount:         1,
			BlockXSize:         505,
			BlockYSize:         4,
			InterlacingPattern: mucog.MUCOGPattern,
			OverviewsMinSize:   geocube.NO_OVERVIEW,
			ResamplingAlg:      0,
			CreationParams:     map[string]string{},
			StorageClass:       0,
		},
	}
)
