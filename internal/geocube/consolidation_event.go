package geocube

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"strings"

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
	ID       string
	DateTime string //"2018-01-01 12:00:00"
	Datasets []ConsolidationDataset
}

// ConsolidationDataset contains all the information on a dataset to consolidate it
type ConsolidationDataset struct {
	URI           string  // "gs://...."
	Subdir        string  // "GTIFF_DIR:1"
	Bands         []int64 // [1, 2, 3]
	Overviews     bool    // true (in case of reconsolidation, do not regenerate overviews if already exist)
	DatasetFormat DataMapping
}

// ConsolidationContainer contains all the information to create the output of the consolidation
type ConsolidationContainer struct {
	URI               string // "gs://bucket/mucog/random_name.TIF"
	DatasetFormat     DataMapping
	CRS               string       // "+init=epsg:XXXX" or WKT
	Transform         [6]float64   // [x0, 10, 0, y_0, 0, -10] Pixels of the image to coordinates in the CRS
	Width, Height     int          // 4096, 4096
	Cutline           string       // POLYGON(coords)
	BandsCount        int          // 3
	BlockXSize        int          // 256
	BlockYSize        int          // 256
	InterleaveBands   bool         // True
	InterleaveRecords bool         // True
	CreateOverviews   bool         // True
	ResamplingAlg     Resampling   // "bilinear"
	Compression       Compression  // "NO", "LOSSLESS", "LOSSY"
	StorageClass      StorageClass // "COLDLINE"
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
		CRS:               crs,
		Transform:         *cell.PixelToCRS,
		Width:             cell.SizeX,
		Height:            cell.SizeY,
		Cutline:           "",
		BandsCount:        len(variable.Bands),
		BlockXSize:        layout.BlockXSize,
		BlockYSize:        layout.BlockYSize,
		InterleaveBands:   params.BandsInterleave,
		InterleaveRecords: true,
		CreateOverviews:   params.Overviews,
		ResamplingAlg:     params.DownsamplingAlg,
		Compression:       params.Compression,
		StorageClass:      params.StorageClass,
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
// Cannot check whether the compression, downsampling_alg or the band interleave changed
func (d *ConsolidationDataset) NeedsReconsolidation(c *ConsolidationContainer) bool {
	if !d.DatasetFormat.Equals(c.DatasetFormat) || d.Overviews != c.CreateOverviews {
		return true
	}

	for _, b := range d.Bands {
		if int(b) > c.BandsCount {
			return true
		}
	}

	str := "Warning !! ConsolidationDataset vs ConsolidationContainer: cannot check: the compression"
	if len(d.Bands) > 1 {
		str += ", the band interleave"
	}
	if d.Overviews {
		str += ", the downsampling_alg"
	}
	fmt.Println(str)
	return false
}

/********************************************************************/
/**                         JOB EVENTS                              */
/********************************************************************/

// JobStatus is the status of a finished step of a job
type JobStatus int32

// Possible status of a finished step
const (
	JobCreated JobStatus = iota
	ConsolidationOrdersPrepared
	PrepareConsolidationOrdersFailed
	SendConsolidationOrdersFailed
	ConsolidationDone
	ConsolidationFailed
	ConsolidationRetried
	ConsolidationRetryFailed
	ConsolidationIndexed
	ConsolidationIndexingFailed
	DatasetsSwapped
	SwapDatasetsFailed
	DeletionOrdersSent
	SendDeletionOrdersFailed
	DeletionDone
	DeletionFailed
	CancelledByUser
	CancellationFailed
	CancellationDone
	RollbackFailed
	RollbackDone
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
	case PrepareConsolidationOrdersFailed,
		SendConsolidationOrdersFailed,
		ConsolidationFailed,
		ConsolidationRetryFailed,
		ConsolidationIndexingFailed,
		CancellationFailed,
		SwapDatasetsFailed,
		SendDeletionOrdersFailed,
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

func (s JobStatus) String() string {
	switch s {
	case JobCreated:
		return "JobCreated"
	case ConsolidationOrdersPrepared:
		return "ConsolidationOrdersPrepared"
	case PrepareConsolidationOrdersFailed:
		return "PrepareConsolidationOrdersFailed"
	case SendConsolidationOrdersFailed:
		return "SendConsolidationOrdersFailed"
	case ConsolidationDone:
		return "ConsolidationDone"
	case ConsolidationFailed:
		return "ConsolidationFailed"
	case ConsolidationRetried:
		return "ConsolidationRetried"
	case ConsolidationRetryFailed:
		return "ConsolidationRetryFailed"
	case ConsolidationIndexed:
		return "ConsolidationIndexed"
	case ConsolidationIndexingFailed:
		return "ConsolidationIndexingFailed"
	case DatasetsSwapped:
		return "DatasetsSwapped"
	case SwapDatasetsFailed:
		return "SwapDatasetsFailed"
	case DeletionOrdersSent:
		return "DeletionOrdersSent"
	case SendDeletionOrdersFailed:
		return "SendDeletionOrdersFailed"
	case CancelledByUser:
		return "CancelledByUser"
	case CancellationFailed:
		return "CancellationFailed"
	case CancellationDone:
		return "CancellationDone"
	case DeletionDone:
		return "DeletionDone"
	case DeletionFailed:
		return "DeletionFailed"
	case RollbackFailed:
		return "RollbackFailed"
	case RollbackDone:
		return "RollbackDone"
	case RetryForced:
		return "RetryForced"
	case Continue:
		return "Continue"
	}
	panic("undefined status")
}
