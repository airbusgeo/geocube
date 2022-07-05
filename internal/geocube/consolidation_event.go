package geocube

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"strings"

	"github.com/airbusgeo/geocube/internal/utils/proj"

	"github.com/airbusgeo/geocube/internal/utils/grid"
)

// Event is a common interface for all job-related events
type Event interface{}

func gobRegisterEvent() {
	gob.Register(TaskEvent{})
	gob.Register(JobEvent{})
}

// MarshalEvent returns bytes representation of a job-related event
func MarshalEvent(evt Event) ([]byte, error) {
	var data bytes.Buffer
	gobRegisterEvent()
	if err := gob.NewEncoder(&data).Encode(&evt); err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}

// UnmarshalEvent returns the event stored in the Reader
func UnmarshalEvent(r io.Reader) (Event, error) {
	var evt Event
	gobRegisterEvent()
	if err := gob.NewDecoder(r).Decode(&evt); err != nil {
		return nil, err
	}

	return evt, nil
}

/********************************************************************/
/**                        TASK EVENTS                              */
/********************************************************************/

// TaskStatus is the status of a finished task
type TaskStatus int32

// Possible status of a finished task
const (
	TaskSuccessful TaskStatus = iota
	TaskFailed
	// TaskIgnored if there is nothing to perform
	TaskIgnored
	// TaskCancelled if the task has been cancelled externally (nothing has been done)
	TaskCancelled
)

// TaskEvent is the event sent from the consolidater when a consolidation task is finished
// TaskEvent implements Event
type TaskEvent struct {
	JobID  string
	TaskID string
	Status TaskStatus
	Error  string
}

// NewTaskEvent returns a new task event
func NewTaskEvent(jobID, taskID string, eventStatus TaskStatus, err error) *TaskEvent {
	serr := ""
	if err != nil {
		serr = err.Error()
	}
	return &TaskEvent{
		JobID:  jobID,
		TaskID: taskID,
		Status: eventStatus,
		Error:  serr,
	}
}

func (ts TaskStatus) String() string {
	switch ts {
	case TaskFailed:
		return "TaskFailed"
	case TaskSuccessful:
		return "TaskSuccessful"
	case TaskCancelled:
		return "TaskCancelled"
	}
	panic("undefined task status")
}

/********************************************************************/
/**                    CONSOLIDATION EVENTS                         */
/********************************************************************/

// ConsolidationEvent is an event sent to the consolidater to start a consolidation task
type ConsolidationEvent struct {
	JobID     string
	TaskID    string
	Records   []ConsolidationRecord
	Container ConsolidationContainer
}

// ConsolidationRecord contains the date and the list of datasets to consolidate
type ConsolidationRecord struct {
	ID         string
	DateTime   string //"2018-01-01 12:00:00"
	Datasets   []ConsolidationDataset
	ValidShape *proj.Shape
}

// ConsolidationDataset contains all the information on a dataset to consolidate it
type ConsolidationDataset struct {
	URI           string  // "gs://...."
	Subdir        string  // "GTIFF_DIR:1"
	Bands         []int64 // [1, 2, 3]
	Overviews     bool    // true (in case of reconsolidation, do not regenerate overviews if already exist)
	DatasetFormat DataMapping
}

const (
	NO_OVERVIEW                = 0
	OVERVIEWS_DEFAULT_MIN_SIZE = -1
)

// ConsolidationContainer contains all the information to create the output of the consolidation
type ConsolidationContainer struct {
	URI                string // "gs://bucket/mucog/random_name.TIF"
	DatasetFormat      DataMapping
	CRS                string       // "+init=epsg:XXXX" or WKT
	Transform          [6]float64   // [x0, 10, 0, y_0, 0, -10] Pixels of the image to coordinates in the CRS
	Width, Height      int          // 4096, 4096
	Cutline            string       // POLYGON(coords)
	BandsCount         int          // 3
	BlockXSize         int          // 256
	BlockYSize         int          // 256
	InterlacingPattern string       // L=0>T>I>P;I>L=1:>T>P (see github.com/airbusgeo/mucog)
	OverviewsMinSize   int          // Maximum width or height of the smallest overview level. 0=NO_OVERVIEW, -1=OVERVIEWS_DEFAULT_MIN_SIZE (=256)
	ResamplingAlg      Resampling   // "bilinear"
	Compression        Compression  // "NO", "LOSSLESS", "LOSSY"
	StorageClass       StorageClass // "COLDLINE"
}

// NewConsolidationContainer initializes a new ConsolidationContainer
func NewConsolidationContainer(URI string, variable *Variable, params *ConsolidationParams, layout *Layout, cell *grid.Cell) (*ConsolidationContainer, error) {
	crs, err := cell.CRS.WKT()
	if err != nil {
		return nil, fmt.Errorf("NewConsolidationContainer: %w", err)
	}

	return &ConsolidationContainer{
		URI: URI,
		DatasetFormat: DataMapping{
			DataFormat: params.DFormat,
			RangeExt:   variable.DFormat.Range,
			Exponent:   params.Exponent,
		},
		CRS:                crs,
		Transform:          *cell.PixelToCRS,
		Width:              cell.SizeX,
		Height:             cell.SizeY,
		Cutline:            "",
		BandsCount:         len(variable.Bands),
		BlockXSize:         layout.BlockXSize,
		BlockYSize:         layout.BlockYSize,
		InterlacingPattern: layout.MucogInterlacingPattern(),
		OverviewsMinSize:   layout.OverviewsMinSize,
		ResamplingAlg:      params.ResamplingAlg,
		Compression:        params.Compression,
		StorageClass:       params.StorageClass,
	}, nil
}

// NewConsolidationDataset initializes a new ConsolidationDataset
func NewConsolidationDataset(d *Dataset) *ConsolidationDataset {
	return &ConsolidationDataset{
		URI:           d.ContainerURI,
		Subdir:        d.ContainerSubDir,
		Overviews:     d.Overviews,
		Bands:         d.Bands,
		DatasetFormat: d.DataMapping,
	}
}

// MarshalConsolidationEvent is used to send a ConsolidationEvent over pubsub
func MarshalConsolidationEvent(evt ConsolidationEvent) ([]byte, error) {
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(&evt); err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}

// UnmarshalConsolidationEvent is used to retrieve a ConsolidationEvent from pubsub
func UnmarshalConsolidationEvent(r io.Reader) (*ConsolidationEvent, error) {
	var evt ConsolidationEvent
	if err := gob.NewDecoder(r).Decode(&evt); err != nil {
		return nil, fmt.Errorf("UnmarshalConsolidationEvent: %w", err)
	}

	return &evt, nil
}

// InGroupOfContainers returns true if the dataset is in the group of containers with the base name
func (d *ConsolidationDataset) InGroupOfContainers(c *ConsolidationContainer) bool {
	return strings.HasPrefix(d.URI, c.URI)
}

// NeedsReconsolidation returns true if the dataset must be reconsolidated
// Cannot check whether the compression, resampling_alg
func (d *ConsolidationDataset) NeedsReconsolidation(c *ConsolidationContainer) bool {
	if !d.DatasetFormat.Equals(c.DatasetFormat) {
		return true
	}

	for _, b := range d.Bands {
		if int(b) > c.BandsCount {
			return true
		}
	}

	fmt.Println("Warning !! ConsolidationDataset vs ConsolidationContainer: cannot check: the compression and the resampling alg")
	return false
}

/********************************************************************/
/**                         JOB EVENTS                              */
/********************************************************************/

//go:generate enumer -json -sql -type JobStatus

// JobStatus defines an event emitted when a step of a job is finished
type JobStatus int32

// Possible status of a finished step
const (
	JobCreated JobStatus = iota
	OrdersPrepared
	PrepareOrdersFailed
	SendOrdersFailed
	ConsolidationDone
	ConsolidationFailed
	ConsolidationRetryFailed
	ConsolidationIndexed
	ConsolidationIndexingFailed

	DatasetsSwapped
	SwapDatasetsFailed
	DeletionStarted
	StartDeletionFailed

	DeletionReady
	DeletionNotReady
	RemovalDone
	DeletionDone
	RemovalFailed
	DeletionFailed

	CancelledByUser
	CancelledByUserForced
	CancellationFailed
	CancellationDone

	RollbackFailed
	RollbackDone

	Retried
	RetryForced

	Continue
)

// JobEvent is the event sent during the job when one of the job steps is finished
// JobEvent implements Event
type JobEvent struct {
	JobID  string
	Status JobStatus
	Error  string
}

func statusWithError(status JobStatus) bool {
	switch status {
	case PrepareOrdersFailed,
		SendOrdersFailed,
		ConsolidationFailed,
		ConsolidationRetryFailed,
		ConsolidationIndexingFailed,
		CancellationFailed,
		SwapDatasetsFailed,
		StartDeletionFailed,
		RemovalFailed,
		DeletionFailed,
		RollbackFailed:
		return true
	}
	return false
}

// NewJobEvent returns a JobEvent and check that all the fields have been filled
// panic if an error is set and it's a non-error status and inversely
func NewJobEvent(jobID string, status JobStatus, err string) *JobEvent {
	if statusWithError(status) != (err != "") {
		panic("Status & error are not compatible")
	}

	return &JobEvent{
		JobID:  jobID,
		Status: status,
		Error:  err,
	}
}
