package geocube

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:generate enumer -json -sql -type JobType -trimprefix JobType
//go:generate enumer -json -sql -type JobState -trimprefix JobState

type JobType int32

const (
	JobTypeCONSOLIDATION JobType = iota
	JobTypeINGESTION
	JobTypeDELETION
)

type JobState int32

const (
	JobStateNEW JobState = iota
	JobStateCREATED
	JobStateCONSOLIDATIONINPROGRESS
	JobStateCONSOLIDATIONDONE
	JobStateCONSOLIDATIONINDEXED
	JobStateCONSOLIDATIONEFFECTIVE
	JobStateDONE

	JobStateFAILED
	JobStateINITIALISATIONFAILED
	JobStateCONSOLIDATIONFAILED
	JobStateCONSOLIDATIONRETRYING
	JobStateCONSOLIDATIONCANCELLING
	JobStateCANCELLATIONFAILED
	JobStateABORTED
	JobStateDONEBUTUNTIDY
)

type LogSeverity string

const (
	DEBUG LogSeverity = "DEBUG"
	INFO  LogSeverity = "INFO"
	WARN  LogSeverity = "WARN"
	ERROR LogSeverity = "ERROR"
)

// JobPayload contains all the information to process a job
type JobPayload struct {
	Layout     string `json:"layout,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	ParamsID   string `json:"params_id,omitempty"`
}

type JobLogs []JobLog

type JobLog struct {
	Severity LogSeverity `json:"severity,omitempty"`
	Msg      string      `json:"message,omitempty"`
	Status   string      `json:"status,omitempty"`
	Date     time.Time   `json:"time,omitempty"`
}

type JobParams interface {
	IsNew() bool
	IsToDelete() bool
	IsDirty() bool
	Deleted()
	Clean()
}

type StepByStepLevel int32

// StepByStepLevel
const (
	StepByStepNone StepByStepLevel = iota
	StepByStepCritical
	StepByStepMajor
	StepByStepAll
)

type LockFlag int32

// LockFlag to state why the dataset is locked
const (
	LockFlagALL LockFlag = iota - 1
	LockFlagINIT
	LockFlagTODELETE
	LockFlagNEW
	LockFlagNB
)

type LockedDatasets struct {
	persistenceState
	newDatasetsID utils.StringSet
}

type Job struct {
	persistenceState
	State          JobState
	ID             string
	Name           string
	Type           JobType
	CreationTime   time.Time
	occTime        time.Time // used for Optimistic Concurrency Control
	LastUpdateTime time.Time
	Payload        JobPayload
	Logs           JobLogs
	ActiveTasks    int
	FailedTasks    int
	StepByStep     int
	Waiting        bool

	// These following fields may not be loaded
	Tasks  []*Task
	Params JobParams

	LockedDatasets [int32(LockFlagNB)]LockedDatasets
}

// NewJob creates a new empty Job with a logger
func NewJob(id string) *Job {
	j := &Job{ID: id}
	return j
}

// NewConsolidationJob creates a new consolidation Job
func NewConsolidationJob(jobName, layout, instanceID string, stepByStep int) *Job {
	id := uuid.New().String()
	j := &Job{
		persistenceState: persistenceStateNEW,
		ID:               id,
		Name:             jobName,
		Type:             JobTypeCONSOLIDATION,
		CreationTime:     time.Now(),
		LastUpdateTime:   time.Now(),
		ActiveTasks:      0,
		FailedTasks:      0,
		Payload: JobPayload{
			Layout:     layout,
			InstanceID: instanceID,
			ParamsID:   id, // By default ParamsID is JobID
		},
		Logs: JobLogs{{
			Severity: "INFO",
			Msg:      "Create Job Consolidation",
			Status:   JobStateNEW.String(),
			Date:     time.Now(),
		}},

		StepByStep: stepByStep,
		Waiting:    false,
	}
	return j
}

func (j *Job) SetParams(params ConsolidationParams) error {
	if !j.IsNew() {
		return fmt.Errorf("job.setParams: cannot set params to a job that is not new")
	}
	j.Params = &params
	params.persistenceState = persistenceStateNEW // Job copies the params and takes ownership
	return nil
}

// ToProtobuf converts a job to protobuf
func (j *Job) ToProtobuf() (*pb.Job, error) {
	creationTime := timestamppb.New(j.CreationTime)
	if err := creationTime.CheckValid(); err != nil {
		return nil, err
	}
	lastUpdateTime := timestamppb.New(j.LastUpdateTime)
	if err := lastUpdateTime.CheckValid(); err != nil {
		return nil, err
	}
	return &pb.Job{
		Id:             j.ID,
		Name:           j.Name,
		Type:           j.Type.String(),
		CreationTime:   creationTime,
		LastUpdateTime: lastUpdateTime,
		State:          j.State.String(),
		ActiveTasks:    int32(j.ActiveTasks),
		FailedTasks:    int32(j.FailedTasks),
		StepByStep:     int32(j.StepByStep),
		Waiting:        j.Waiting,
		Logs:           j.Logs.toSliceString(),
	}, nil
}

// Clean overrides persistentState.Clean and set the status Clean to the job
// "all" also sets the status to the locked datasets and all its tasks
func (j *Job) Clean(all bool) {
	if all {
		for i := range j.LockedDatasets {
			j.LockedDatasets[i].Clean()
		}
		for _, t := range j.Tasks {
			t.Clean()
		}
	}
	j.occTime = j.LastUpdateTime
	j.persistenceState.Clean()
}

// dirty overrides persistentState.dirty
func (j *Job) dirty() {
	j.LastUpdateTime = time.Now()
	j.persistenceState.dirty()
}

// ToDelete sets the status ToDelete to the job iif tasks are also ToDelete or empty
// If all, also delete tasks
// Return success
func (j *Job) ToDelete(all bool) bool {
	for _, t := range j.Tasks {
		if t.IsActive() {
			if !all {
				return false
			}
			t.toDelete()
		}
	}
	j.ReleaseDatasets(LockFlagALL)
	j.toDelete()
	return true
}

// OCCTime returns a timestamp used to do Optimistic Concurrency Control
func (j *Job) OCCTime() time.Time {
	return j.occTime
}

// LogMsg updates and append the log status of Job
func (j *Job) LogMsg(severity LogSeverity, msg string) {
	j.Logs = append(j.Logs, JobLog{
		Severity: severity,
		Msg:      msg,
		Status:   j.State.String(),
		Date:     time.Now(),
	})
	j.dirty()
}

// LogMsgf updates and append the log status of Job
func (j *Job) LogMsgf(severity LogSeverity, msg string, args ...interface{}) {
	j.LogMsg(severity, fmt.Sprintf(msg, args...))
}

// LogErr updates and append the error status
func (j *Job) LogErr(err string) {
	if err != "" {
		j.Logs = append(j.Logs, JobLog{
			Severity: ERROR,
			Msg:      err,
			Status:   j.State.String(),
			Date:     time.Now(),
		})
	}
	j.dirty()
}

/***************************************************/
/**                  TRIGGERS                     **/
/***************************************************/

// Trigger handles the event and change the state of the job
// Only returns UnhandledEvent
func (j *Job) Trigger(evt JobEvent) error {
	handled := false
	switch j.Type {
	case JobTypeCONSOLIDATION:
		handled = j.triggerConsolidation(evt)
	case JobTypeDELETION:
		handled = j.triggerDeletion(evt)
	case JobTypeINGESTION:
		handled = j.triggerIngestion(evt)
	}
	if handled {
		if j.Waiting {
			j.LogMsg(INFO, "New state: "+j.State.String()+": waiting for user action")
		} else {
			j.LogMsg(INFO, "New state: "+j.State.String())

		}
		return nil
	}
	return NewUnhandledEvent("Job " + j.ID + ": Unable to trigger " + evt.Status.String() + " (current state=" + j.State.String() + ")")
}

func (j *Job) triggerConsolidation(evt JobEvent) bool {
	if evt.Status == Continue {
		if j.Waiting {
			j.Waiting = false
			j.dirty()
			return true
		}
	}

	switch j.State {
	case JobStateNEW:
		switch evt.Status {
		case RetryForced:
			return true
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case JobCreated:
			return j.changeState(JobStateCREATED)
		}
	case JobStateCREATED:
		switch evt.Status {
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case RetryForced:
			return true
		case PrepareConsolidationOrdersFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateINITIALISATIONFAILED)
		case ConsolidationOrdersPrepared:
			return j.changeState(JobStateCONSOLIDATIONINPROGRESS)
		}
	case JobStateCONSOLIDATIONINPROGRESS:
		switch evt.Status {
		case RetryForced:
			return j.changeState(JobStateCONSOLIDATIONRETRYING)
		case CancelledByUser, CancelledByUserForced:
			j.LogErr("Cancelled by user")
			return j.changeState(JobStateCONSOLIDATIONCANCELLING)
		case ConsolidationFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case SendConsolidationOrdersFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case ConsolidationDone:
			return j.changeState(JobStateCONSOLIDATIONDONE)
		}
	case JobStateCONSOLIDATIONDONE:
		switch evt.Status {
		case RetryForced:
			return true
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case ConsolidationIndexingFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case ConsolidationIndexed:
			return j.changeState(JobStateCONSOLIDATIONINDEXED)
		}
	case JobStateCONSOLIDATIONINDEXED:
		switch evt.Status {
		case RetryForced:
			return true
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case SwapDatasetsFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case DatasetsSwapped:
			return j.changeState(JobStateCONSOLIDATIONEFFECTIVE)
		}
	case JobStateCONSOLIDATIONEFFECTIVE:
		switch evt.Status {
		case RetryForced:
			return true
		case DeletionFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateDONEBUTUNTIDY)
		case DeletionDone:
			return j.changeState(JobStateDONE)
		}
	case JobStateDONE:
		return false

	case JobStateDONEBUTUNTIDY:
		switch evt.Status {
		case RetryForced:
			return j.changeState(JobStateCONSOLIDATIONEFFECTIVE)
		case DeletionFailed:
			j.LogErr(evt.Error)
			return j.changeState(JobStateDONEBUTUNTIDY)
		case DeletionDone:
			return j.changeState(JobStateDONE)
		}
	case JobStateCONSOLIDATIONCANCELLING:
		switch evt.Status {
		case RetryForced:
			return true
		case CancellationFailed:
			return j.changeState(JobStateCANCELLATIONFAILED)
		case CancellationDone:
			return j.changeState(JobStateABORTED)
		}
	case JobStateCANCELLATIONFAILED:
		switch evt.Status {
		case ConsolidationRetried, RetryForced:
			return j.changeState(JobStateCONSOLIDATIONCANCELLING)
		}
	case JobStateINITIALISATIONFAILED:
		switch evt.Status {
		case ConsolidationRetried, RetryForced:
			return j.changeState(JobStateCREATED)
		case CancelledByUser, CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		}
	case JobStateCONSOLIDATIONFAILED:
		switch evt.Status {
		case ConsolidationRetried, RetryForced:
			return j.changeState(JobStateCONSOLIDATIONRETRYING)
		case CancelledByUser, CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		}
	case JobStateABORTED:
		switch evt.Status {
		case RetryForced:
			return true
		case RollbackDone:
			return j.changeState(JobStateFAILED)
		}

	case JobStateCONSOLIDATIONRETRYING:
		switch evt.Status {
		case ConsolidationRetryFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case ConsolidationOrdersPrepared:
			return j.changeState(JobStateCONSOLIDATIONINPROGRESS)
		}
	case JobStateFAILED:
		return false
	default:
		panic("trigger: Unknown state")
	}

	return false
}

func (j *Job) triggerDeletion(evt JobEvent) bool {
	panic("TODO Deletion Not Implemented")
}

func (j *Job) triggerIngestion(evt JobEvent) bool {
	panic("TODO Ingestion Not Implemented")
}

func (j *Job) changeState(newState JobState) bool {
	j.State = newState
	j.LogMsgf(DEBUG, "Update status to : %s", newState.String())
	switch j.State {
	case JobStateCONSOLIDATIONINPROGRESS, JobStateCONSOLIDATIONEFFECTIVE, JobStateABORTED:
		// Before consolidation, before deletion, before rollback
		j.Waiting = j.StepByStep >= int(StepByStepCritical)

	case JobStateCREATED, JobStateCONSOLIDATIONDONE, JobStateCONSOLIDATIONCANCELLING, JobStateCONSOLIDATIONRETRYING:
		j.Waiting = j.StepByStep >= int(StepByStepMajor)

	default:
		// JobStateNEW, JobStateCONSOLIDATIONINDEXED, JobStateDONE, JobStateDONEBUTUNTIDY, JobStateCONSOLIDATIONFAILED, JobStateINITIALISATIONFAILED, JobStateCANCELLATIONFAILED, JobStateFAILED:
		j.Waiting = j.StepByStep >= int(StepByStepAll)
	}
	j.dirty()
	return true
}

/***************************************************/
/**                   TASKS                       **/
/***************************************************/

// CreateConsolidationTask adds a new consolidation task with the event provided
func (j *Job) CreateConsolidationTask(evt ConsolidationEvent) error {
	t, err := newConsolidationTask(evt)
	if err == nil {
		j.ActiveTasks++
		j.Tasks = append(j.Tasks, t)
		j.dirty()
	}
	return err
}

func taskStateFromStatus(status TaskStatus) TaskState {
	switch status {
	case TaskFailed:
		return TaskStateFAILED
	case TaskIgnored, TaskCancelled:
		return TaskStateCANCELLED
	case TaskSuccessful:
		return TaskStateDONE
	}
	panic("Unknown status: " + status.String())
}

// UpdateTask updates the status of the task depending on the event
// The task must exists and in pending state
func (j *Job) UpdateTask(evt TaskEvent) error {
	// Get the task
	task := j.task(evt.TaskID)
	if task == nil {
		return NewEntityNotFound("Task", "id", evt.TaskID, "")
	}

	// Get the new state (return if it's the same)
	newState := taskStateFromStatus(evt.Status)
	if newState == task.State {
		return nil
	}

	// If the state is different but the job cannot handle task events, returns !
	if j.State != JobStateCONSOLIDATIONINPROGRESS && j.State != JobStateCONSOLIDATIONCANCELLING {
		return NewUnhandledEvent("Job %s Task %s Status %s", j.ID, evt.TaskID, evt.Status.String())
	}

	switch task.State {
	case TaskStateDONE:
		// The task has already been reported as Successful and now it is cancelled, ignored or has failed (?!)
		return NewUnhandledEvent("Job %s Task %s Status %s", j.ID, evt.TaskID, evt.Status.String())
	case TaskStateFAILED, TaskStateCANCELLED:
		if newState != TaskStateDONE {
			return nil
		}
		// The task has already been reported as failed or cancelled, but it's not too late to tag it as successful !
	}

	// Change the task state
	j.setTaskState(task, newState)

	if newState == TaskStateFAILED {
		j.LogErr("Task " + evt.TaskID + " failed: " + evt.Error)
	}

	return nil
}

// ResetAllTasks sets the pending state to all the tasks and the job status
func (j *Job) ResetAllTasks() {
	for _, t := range j.Tasks {
		j.setTaskState(t, TaskStatePENDING)
	}
}

// CancelTask sets the cancel state to the task with the given index
func (j *Job) CancelTask(index int) {
	j.setTaskState(j.Tasks[index], TaskStateCANCELLED)
}

// DeleteTask set the status ToDelete to one task
func (j *Job) DeleteTask(index int) {
	task := j.Tasks[index]
	if task.IsActive() /*persistentState*/ {
		j.setTaskState(task, TaskStateCANCELLED)
	}
	task.toDelete()
}

// DeleteAllTasks set the status ToDelete to all the tasks
func (j *Job) DeleteAllTasks() {
	for i := range j.Tasks {
		j.DeleteTask(i)
	}
}

func (j *Job) updateTaskCounters(state TaskState, inc int) {
	switch state {
	case TaskStatePENDING:
		j.ActiveTasks += inc
		j.dirty()
	case TaskStateFAILED:
		j.FailedTasks += inc
		j.dirty()
	}
	if j.ActiveTasks < 0 {
		fmt.Printf("Active tasks number cannot be negative")
		panic("Active tasks number cannot be negative")
	}
	if j.FailedTasks < 0 {
		fmt.Printf("Failed tasks number cannot be negative")
		panic("Failed tasks number cannot be negative")
	}
}

func (j *Job) setTaskState(task *Task, newState TaskState) bool {
	oldState := task.State
	if task.setState(newState) {
		j.updateTaskCounters(oldState, -1)
		j.updateTaskCounters(newState, 1)
		return true
	}
	return false
}

// task retrieves unefficiently the task but usually, there is only one task
func (j *Job) task(taskID string) *Task {
	for _, t := range j.Tasks {
		if t.ID == taskID {
			return t
		}
	}
	return nil
}

/***************************************************/
/**                  Payload                      **/
/***************************************************/

// Value implements the driver.Valuer interface for a jobPayload. This method
// simply returns the JSON-encoded representation of the struct.
func (jp JobPayload) Value() (driver.Value, error) {
	b, err := json.Marshal(jp)
	return string(b), err
}

// Scan implements the sql.Scanner interface for a jobPayload. This method
// simply decodes a JSON-encoded value into the struct fields.
func (jp *JobPayload) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &jp)
}

/***************************************************/
/**                  JobLogs                      **/
/***************************************************/

func (jl JobLogs) Len() int           { return len(jl) }
func (jl JobLogs) Less(i, j int) bool { return jl[i].Date.Before(jl[j].Date) }
func (jl JobLogs) Swap(i, j int)      { jl[i], jl[j] = jl[j], jl[i] }

func (jl JobLogs) toSliceString() []string {
	sort.Sort(jl)
	var result []string
	for _, log := range jl {
		result = append(result, fmt.Sprintf("%s - %s | Status: %s - Message: %s", log.Date.Format(time.RFC3339Nano), log.Severity, log.Status, log.Msg))
	}
	return result
}

// Value implements the driver.Valuer interface for a JobLogs. This method
// simply returns the JSON-encoded representation of the struct.
func (jl JobLogs) Value() (driver.Value, error) {
	b, err := json.Marshal(jl)
	return string(b), err
}

// Scan implements the sql.Scanner interface for a JobLogs. This method
// simply decodes a JSON-encoded value into the struct fields.
func (jl *JobLogs) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &jl)
}

/***************************************************/
/**              LockedDataSets                   **/
/***************************************************/

// LockDatasets set the status New to the lock of the datasets (flag = LockFlagINIT/LockFlagTODELETE/LockFlagNEW) and adds the datasets
func (j *Job) LockDatasets(datasetsID []string, flag LockFlag) {
	if flag == LockFlagALL || len(datasetsID) == 0 {
		return
	}
	j.LockedDatasets[flag].dirty()
	if j.LockedDatasets[flag].newDatasetsID == nil {
		j.LockedDatasets[flag].newDatasetsID = utils.StringSet{}
	}
	for _, datasetID := range datasetsID {
		j.LockedDatasets[flag].newDatasetsID.Push(datasetID)
	}
}

// ReleaseDatasets set the status ToDelete to the lock of the datasets (any flags)
func (j *Job) ReleaseDatasets(flag LockFlag) {
	if flag == LockFlagALL {
		for f := range j.LockedDatasets {
			j.LockedDatasets[f].toDelete()
		}
	} else {
		j.LockedDatasets[flag].toDelete()
	}
}

// Clean overrides persistenceState.Clean()
func (l *LockedDatasets) Clean() {
	l.newDatasetsID = nil
	l.persistenceState.Clean()
}

// NewIDs returns the ID of the datasets to be locked
func (l LockedDatasets) NewIDs() []string {
	return l.newDatasetsID.Slice()
}
