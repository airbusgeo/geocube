package svc_test

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/twpayne/go-geom"
	"go.uber.org/zap"

	geomGeojson "github.com/twpayne/go-geom/encoding/geojson"

	"github.com/stretchr/testify/mock"

	mocksDB "github.com/airbusgeo/geocube/interface/database/mocks"
	mocksMessaging "github.com/airbusgeo/geocube/interface/messaging/mocks"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/svc"
	"github.com/airbusgeo/godal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Consolidater", func() {

	var (
		datasetsToUse  []*svc.CsldDataset
		containerToUse geocube.ConsolidationContainer

		returnedNeed     bool
		returnedDatasets []string
	)

	BeforeEach(func() {
		godal.RegisterAll()
	})

	var (
		itShouldNeedConsolidation = func() {
			It("it should need consolidation", func() {
				Expect(returnedNeed).To(BeTrue())
			})
		}
		itShouldNotNeedConsolidation = func() {
			It("it should not need consolidation", func() {
				Expect(returnedNeed).To(BeFalse())
			})
		}
		itShouldNotReturnDatasets = func() {
			It("it should not return a dataset", func() {
				Expect(returnedDatasets).To(BeEmpty())
			})
		}
		itShouldReturnDataset = func(id string) {
			It("it should return a dataset", func() {
				Expect(returnedDatasets).To(Equal([]string{id}))
			})
		}
		itShouldReturnNDatasets = func(n int) {
			It("it should return N datasets", func() {
				Expect(len(returnedDatasets)).To(Equal(n))
			})
		}
	)

	Describe("CsldPrepareOrdersNeedReconsolidation", func() {

		JustBeforeEach(func() {
			returnedNeed, returnedDatasets = svc.CsldPrepareOrdersNeedReconsolidation(&datasetsToUse, &containerToUse)
		})

		Context("one basic dataset", func() {
			BeforeEach(func() {
				datasetsToUse = datasetNotConsolidated
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with no overview", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_123_NO
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with another dataformat", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedI_123_O
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with other bands", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_234_O
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_123_O
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnDataset(datasetConsolidatedF_123_O[0].Event.URI)
		})

		Context("several identical consolidated datasets", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnDataset(datasetsConsolidatedF_123_O[0].Event.URI)
		})

		Context("several identical consolidated datasets in two containers", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O_2
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnNDatasets(2)
		})

		Context("several identical and different consolidated datasets in two containers", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O_3
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldReturnNDatasets(2)
		})
	})

	Describe("ConsolidateFromRecords", func() {

		var (
			ctx = context.Background()

			mockDatabase               = new(mocksDB.GeocubeBackend)
			mockEventPublisher         = new(mocksMessaging.Publisher)
			mockConsolidationPublisher = new(mocksMessaging.Publisher)
			service                    *svc.Service

			jobToUse  *geocube.Job
			recordIDS []string

			returnedError error

			datasetListReturned []string
			listErrorReturned   error

			geocubeTxBackendReturned      = new(mocksDB.GeocubeTxBackend)
			geocubeTxBackendErrorReturned error

			rollbackErrorReturned error
			commitErrorReturned   error

			variableToReturned      *geocube.Variable
			variableErrorToReturned error

			consolidationParamReturned      *geocube.ConsolidationParams
			consolidationParamErrorReturned error

			createJobErrorReturned error

			createConsolidationParamErrorReturned error
			lockDatasetErrorReturned              error
			publishErrorReturned                  error
		)

		BeforeEach(func() {
			var err error
			service, err = svc.New(ctx, mockDatabase, mockEventPublisher, mockConsolidationPublisher, os.TempDir(), os.TempDir(), 1)
			if err != nil {
				panic(err)
			}

			jobToUse, _ = geocube.NewConsolidationJob("test_consolidation", "layoutID", "instanceID", geocube.ExecutionAsynchronous)
			jobToUse.Tasks = append(jobToUse.Tasks, &geocube.Task{
				ID:      "task1",
				State:   geocube.TaskStateDONE,
				Payload: nil,
			})

			recordIDS = []string{"record1", "record2"}

			datasetListReturned = []string{"dataset1", "dataset2"}
			listErrorReturned = nil

			geocubeTxBackendErrorReturned = nil
			rollbackErrorReturned = nil
			commitErrorReturned = nil

			variableToReturned = &geocube.Variable{
				ID:   "var1",
				Name: "var1",
				DFormat: geocube.DataFormat{
					DType:  geocube.DTypeFromString("uint16"),
					NoData: 0,
					Range: geocube.Range{
						Min: 0,
						Max: 5000,
					},
				},
				ConsolidationParams: geocube.ConsolidationParams{
					DFormat: geocube.DataFormat{
						DType:  geocube.DTypeFromString("uint8"),
						NoData: 0,
						Range: geocube.Range{
							Min: 0,
							Max: 255,
						},
					},
					Exponent:         0,
					Compression:      0,
					OverviewsMinSize: geocube.NO_OVERVIEW,
					BandsInterleave:  false,
					StorageClass:     0,
				},
			}
			variableErrorToReturned = nil

			consolidationParamReturned = &geocube.ConsolidationParams{
				DFormat: geocube.DataFormat{
					DType:  geocube.DTypeFromString("uint8"),
					NoData: 0,
					Range: geocube.Range{
						Min: 0,
						Max: 255,
					},
				},
				Exponent:         0,
				Compression:      0,
				OverviewsMinSize: geocube.NO_OVERVIEW,
				BandsInterleave:  false,
				StorageClass:     0,
			}
			consolidationParamErrorReturned = nil

			createJobErrorReturned = nil
			createConsolidationParamErrorReturned = nil
			lockDatasetErrorReturned = nil
			publishErrorReturned = nil

		})

		JustBeforeEach(func() {
			ctx := log.WithFields(ctx, zap.String("job", jobToUse.ID))
			mockDatabase.On("ListActiveDatasetsID", ctx, jobToUse.Payload.InstanceID, recordIDS, mock.Anything, mock.Anything, mock.Anything).Return(datasetListReturned, listErrorReturned)
			mockDatabase.On("StartTransaction", ctx).Return(geocubeTxBackendReturned, geocubeTxBackendErrorReturned)
			geocubeTxBackendReturned.On("Rollback").Return(rollbackErrorReturned)
			geocubeTxBackendReturned.On("Commit").Return(commitErrorReturned)
			geocubeTxBackendReturned.On("ReadVariableFromInstanceID", ctx, jobToUse.Payload.InstanceID).Return(variableToReturned, variableErrorToReturned)
			geocubeTxBackendReturned.On("ReadConsolidationParams", ctx, mock.Anything).Return(consolidationParamReturned, consolidationParamErrorReturned)
			geocubeTxBackendReturned.On("CreateJob", ctx, jobToUse).Return(createJobErrorReturned)
			geocubeTxBackendReturned.On("CreateConsolidationParams", ctx, jobToUse.Payload.ParamsID, mock.Anything).Return(createConsolidationParamErrorReturned)
			geocubeTxBackendReturned.On("LockDatasets", ctx, jobToUse.ID, mock.Anything, mock.Anything).Return(lockDatasetErrorReturned)
			mockEventPublisher.On("Publish", ctx, mock.Anything).Return(publishErrorReturned)

			returnedError = service.ConsolidateFromRecords(ctx, jobToUse, recordIDS)
		})

		var (
			itShouldNotReturnAnError = func() {
				It("should not return an error", func() {
					Expect(returnedError).To(BeNil())
					for _, log := range jobToUse.Logs {
						Expect(log.Severity).NotTo(Equal("ERROR"))
					}
				})
			}

			itShouldCreateJobState = func(status string) {
				It("should create job state", func() {
					Expect(jobToUse.State.String()).To(Equal(status))

				})
			}
		)

		Context("default new Job", func() {
			itShouldNotReturnAnError()
			itShouldCreateJobState("NEW")
		})

		Context("default created Job", func() {
			var (
				findRecordReturned      []*geocube.Record
				findRecordErrorReturned error
			)
			BeforeEach(func() {
				jobToUse.State = geocube.JobStateCREATED
				findRecordReturned = []*geocube.Record{{
					ID:   "recordReturnedID",
					Name: "recordReturned",
					Time: time.Time{},
					Tags: nil,
					AOI:  geocube.AOI{},
				}}
				findRecordErrorReturned = nil
				var inputFeature geomGeojson.Feature
				input := `{"type":"Feature","properties":{},"geometry":{"type":"MultiPolygon","coordinates":[[[[-0.17578125,44.715513732021336],[4.39453125,44.715513732021336],[4.39453125,49.55372551347579],[-0.17578125,49.55372551347579],[-0.17578125,44.715513732021336]]],[[[6.328125,44.276671273775186],[8.96484375,44.276671273775186],[8.96484375,47.338822694822],[6.328125,47.338822694822],[6.328125,44.276671273775186]]]]}}`
				err := json.Unmarshal([]byte(input), &inputFeature)
				if err != nil {
					panic(err)
				}
				ctx := log.WithFields(ctx, zap.String("job", jobToUse.ID))
				multipolygon := geom.NewMultiPolygonFlat(inputFeature.Geometry.Layout(), inputFeature.Geometry.FlatCoords(), inputFeature.Geometry.Endss())
				geocubeTxBackendReturned.On("FindRecords", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(findRecordReturned, findRecordErrorReturned)
				geocubeTxBackendReturned.On("GetDatasetsGeometryUnion", ctx, mock.Anything).Return(multipolygon, nil)
				geocubeTxBackendReturned.On("ReadLayout", ctx, mock.Anything).Return(&geocube.Layout{
					Name:      "myLayout",
					GridFlags: []string{},
					GridParameters: geocube.Metadata{
						"grid":       "singlecell",
						"proj":       "epsg",
						"crs":        "4326",
						"resolution": "0.0001716613769531200127",
					},
					BlockXSize: 256,
					BlockYSize: 256,
					MaxRecords: 10,
				}, nil)
				geocubeTxBackendReturned.On("FindDatasets", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*geocube.Dataset{}, nil)
				geocubeTxBackendReturned.On("ReleaseDatasets", ctx, mock.Anything, mock.Anything).Return(nil)
				geocubeTxBackendReturned.On("UpdateJob", ctx, mock.Anything).Return(nil)
			})
			itShouldNotReturnAnError()
			itShouldCreateJobState("CREATED")
		})

		XContext("default in progress job", func() {
			//TODO WIP
			BeforeEach(func() {
				jobToUse.State = geocube.JobStateCONSOLIDATIONINPROGRESS
			})
		})
	})
})
