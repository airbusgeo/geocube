package svc_test

import (
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/svc"
)

var (
	unconsolidatedBaseName = "gs://BaseName1/"

	consolidatedBaseName = "gs://BaseName2"

	dataMappingF = geocube.DataMapping{
		DataFormat: geocube.DataFormat{
			DType:  geocube.DTypeFLOAT32,
			NoData: 0,
			Range:  geocube.Range{Min: 0, Max: 1},
		},
		RangeExt: geocube.Range{Min: 0, Max: 1},
		Exponent: 0,
	}

	dataMappingI = geocube.DataMapping{
		DataFormat: geocube.DataFormat{
			DType:  geocube.DTypeINT16,
			NoData: 0,
			Range:  geocube.Range{Min: 0, Max: 10000},
		},
		RangeExt: geocube.Range{Min: 0, Max: 1},
		Exponent: 0,
	}

	datasetNotConsolidated = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           unconsolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     false,
			DatasetFormat: dataMappingF,
		}},
	}

	datasetConsolidatedF_123_O = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
	}

	datasetConsolidatedF_234_O = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{2, 3, 4},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
	}

	datasetConsolidatedF_123_NO = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     false,
			DatasetFormat: dataMappingF,
		}},
	}

	datasetConsolidatedI_123_O = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingI,
		}},
	}

	datasetsConsolidatedF_123_O = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:2",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
	}
	datasetsConsolidatedF_123_O_2 = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "2.tiff",
			Subdir:        "GTIFF_DIR:2",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
	}
	datasetsConsolidatedF_123_O_3 = []*svc.CsldDataset{
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "1.tiff",
			Subdir:        "GTIFF_DIR:1",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
		{Event: geocube.ConsolidationDataset{
			URI:           consolidatedBaseName + "2.tiff",
			Subdir:        "GTIFF_DIR:2",
			Bands:         []int64{1, 2, 3},
			Overviews:     true,
			DatasetFormat: dataMappingF,
		}},
		datasetNotConsolidated[0],
	}
	containerF_3_O = geocube.ConsolidationContainer{
		URI:              consolidatedBaseName,
		DatasetFormat:    dataMappingF,
		BandsCount:       3,
		OverviewsMinSize: geocube.OVERVIEWS_DEFAULT_MIN_SIZE,
	}
)
