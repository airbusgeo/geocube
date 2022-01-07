package svc_test

import (
	"context"
	"os"
	"path"

	mocksDB "github.com/airbusgeo/geocube/interface/database/mocks"
	mocksMessaging "github.com/airbusgeo/geocube/interface/messaging/mocks"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/stretchr/testify/mock"

	"github.com/airbusgeo/geocube/internal/svc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("csldCancel", func() {

	var (
		ctx = context.Background()

		mockDatabase               = new(mocksDB.GeocubeBackend)
		mockEventPublisher         = new(mocksMessaging.Publisher)
		mockConsolidationPublisher = new(mocksMessaging.Publisher)

		jobIDToUse    string
		returnedError error

		readJobReturned      *geocube.Job
		readJobErrorReturned error

		geocubeTxBackendReturned      = new(mocksDB.GeocubeTxBackend)
		geocubeTxBackendErrorReturned error
		rollbackErrorReturned         error
		commitErrorReturned           error
		updateJobErrorToReturned      error
		readTasksReturned             []*geocube.Task
		readTasksErrorReturned        error
		updateTaskErrorReturned       error
		publishErrorReturned          error

		service *svc.Service
	)

	BeforeEach(func() {
		var err error
		service, err = svc.New(ctx, mockDatabase, mockEventPublisher, mockConsolidationPublisher, os.TempDir(), os.TempDir(), 1)
		if err != nil {
			panic(err)
		}

		returnedError = nil
		jobIDToUse = "myJobID"

		readJobReturned = &geocube.Job{
			State:       geocube.JobStateCONSOLIDATIONINPROGRESS,
			ID:          "jobID",
			Name:        "",
			Type:        geocube.JobTypeCONSOLIDATION,
			ActiveTasks: 2,
			FailedTasks: 0,
			Tasks: []*geocube.Task{
				{
					ID:    "task1",
					State: geocube.TaskStatePENDING,
				}, {
					ID:    "task2",
					State: geocube.TaskStatePENDING,
				},
			},
			ExecutionLevel: geocube.ExecutionAsynchronous,
		}
		readJobErrorReturned = nil

		geocubeTxBackendErrorReturned = nil
		rollbackErrorReturned = nil
		commitErrorReturned = nil
		updateJobErrorToReturned = nil

		readTasksReturned = []*geocube.Task{
			{ID: "task1", State: geocube.TaskStatePENDING},
			{ID: "task2", State: geocube.TaskStatePENDING},
		}
		readTasksErrorReturned = nil
		updateTaskErrorReturned = nil
		publishErrorReturned = nil

	})

	JustBeforeEach(func() {
		mockDatabase.On("ReadJob", ctx, jobIDToUse).Return(readJobReturned, readJobErrorReturned)
		mockDatabase.On("StartTransaction", ctx).Return(geocubeTxBackendReturned, geocubeTxBackendErrorReturned)
		mockDatabase.On("ReadTasks", ctx, readJobReturned.ID, mock.Anything).Return(readTasksReturned, readTasksErrorReturned)
		geocubeTxBackendReturned.On("Rollback").Return(rollbackErrorReturned)
		geocubeTxBackendReturned.On("Commit").Return(commitErrorReturned)
		geocubeTxBackendReturned.On("UpdateJob", ctx, readJobReturned).Return(updateJobErrorToReturned)
		geocubeTxBackendReturned.GeocubeBackend.On("UpdateTask", ctx, mock.Anything).Return(updateTaskErrorReturned)
		mockEventPublisher.On("Publish", ctx, mock.Anything).Return(publishErrorReturned)
		returnedError = service.CancelJob(ctx, jobIDToUse, false)
	})

	var (
		itShouldNotReturnAnError = func() {
			It("should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}

		itShouldWriteCancelledFile = func() {
			It("should write cancelled file", func() {
				_, errFiTask1 := os.Stat(path.Join(os.TempDir(), "jobID_task1"))
				_, errFiTask2 := os.Stat(path.Join(os.TempDir(), "jobID_task2"))

				Expect(errFiTask1).NotTo(Equal(os.IsNotExist(errFiTask1)))
				Expect(errFiTask2).NotTo(Equal(os.IsNotExist(errFiTask2)))
			})
		}
	)

	AfterEach(func() {
		os.Remove(path.Join(os.TempDir(), "jobID_task1"))
		os.Remove(path.Join(os.TempDir(), "jobID_task2"))
	})

	Context("default", func() {
		itShouldNotReturnAnError()
		itShouldWriteCancelledFile()
	})
})
