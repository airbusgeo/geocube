package image_test

import "github.com/airbusgeo/geocube/internal/geocube"

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
						URI:       "file://test_data/image_20180812.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
					DType:  geocube.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:               `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:         [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:             256,
			Height:            256,
			Cutline:           "",
			BandsCount:        1,
			BlockXSize:        256,
			BlockYSize:        256,
			InterleaveBands:   false,
			InterleaveRecords: true,
			OverviewsMinSize:  geocube.NO_OVERVIEW,
			ResamplingAlg:     geocube.ResamplingNEAR,
			Compression:       geocube.CompressionLOSSLESS,
			StorageClass:      geocube.StorageClassSTANDARD,
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
						URI:       "file://test_data/image_20180812_1.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
								Range:  geocube.Range{Min: 0, Max: 1},
								NoData: 0,
							},
							RangeExt: geocube.Range{Min: 0, Max: 1},
							Exponent: 1,
						},
					}, {
						URI:       "file://test_data/image_20180812_2.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
					DType:  geocube.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:               `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:         [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:             256,
			Height:            256,
			Cutline:           "",
			BandsCount:        1,
			BlockXSize:        256,
			BlockYSize:        256,
			InterleaveBands:   false,
			InterleaveRecords: true,
			OverviewsMinSize:  geocube.NO_OVERVIEW,
			ResamplingAlg:     geocube.ResamplingNEAR,
			Compression:       geocube.CompressionLOSSLESS,
			StorageClass:      geocube.StorageClassSTANDARD,
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
						URI:       "file://test_data/image_20180812.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
						URI:       "file://test_data/image_20180824.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
					DType:  geocube.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:               `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:         [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:             256,
			Height:            256,
			Cutline:           "",
			BandsCount:        1,
			BlockXSize:        256,
			BlockYSize:        256,
			InterleaveBands:   false,
			InterleaveRecords: true,
			OverviewsMinSize:  geocube.NO_OVERVIEW,
			ResamplingAlg:     geocube.ResamplingNEAR,
			Compression:       geocube.CompressionLOSSLESS,
			StorageClass:      geocube.StorageClassSTANDARD,
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
						URI:       "file://test_data/image_20180812.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
					DType:  geocube.DTypeFLOAT64,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{Min: 0, Max: 1},
				Exponent: 1,
			},
			CRS:               `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:         [6]float64{450560.0, 200.0, 0.0, 6266880.0, 0.0, -200.0},
			Width:             256,
			Height:            256,
			Cutline:           "",
			BandsCount:        1,
			BlockXSize:        256,
			BlockYSize:        256,
			InterleaveBands:   false,
			InterleaveRecords: true,
			OverviewsMinSize:  geocube.NO_OVERVIEW,
			ResamplingAlg:     geocube.ResamplingNEAR,
			Compression:       geocube.CompressionLOSSLESS,
			StorageClass:      geocube.StorageClassSTANDARD,
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
						URI:       "file://test_data/image_20180812.tif",
						Subdir:    "",
						Bands:     []int64{1},
						Overviews: false,
						DatasetFormat: geocube.DataMapping{
							DataFormat: geocube.DataFormat{
								DType:  geocube.DTypeFLOAT32,
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
					DType:  geocube.DTypeFLOAT32,
					Range:  geocube.Range{Min: 0, Max: 1},
					NoData: 0,
				},
				RangeExt: geocube.Range{
					Min: 0,
					Max: 1,
				},
				Exponent: 0,
			},
			CRS:               `PROJCS["WGS 84 / UTM zone 32N",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",9],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32632"]]`,
			Transform:         [6]float64{460943.9866, 200.19801980198008, 0, 6.2551182875e+06, 0, -200.19900497512438},
			Width:             505,
			Height:            201,
			Cutline:           "",
			BandsCount:        1,
			BlockXSize:        505,
			BlockYSize:        4,
			InterleaveBands:   false,
			InterleaveRecords: true,
			OverviewsMinSize:  geocube.NO_OVERVIEW,
			ResamplingAlg:     0,
			Compression:       0,
			StorageClass:      0,
		},
	}
)
