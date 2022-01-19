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
	JobStateCONSOLIDATIONFAILED
	JobStateCONSOLIDATIONRETRYING
	JobStateCONSOLIDATIONCANCELLING

	JobStateDELETIONINPROGRESS
	JobStateDELETIONEFFECTIVE
	JobStateDELETIONFAILED

	JobStateDONE

	JobStateFAILED
	JobStateINITIALISATIONFAILED
	JobStateCANCELLATIONFAILED
	JobStateABORTED
	JobStateROLLBACKFAILED
	JobStateDONEBUTUNTIDY
)

type JobStateInfo struct {
	Level       ExecutionLevel
	RetryForced bool // if event = RetryForced, retry the current state (other behaviors can be defined in the trigger...() functions )
}

var jobStateInfo = map[JobState]JobStateInfo{
	JobStateNEW:                     {StepByStepAll, true},
	JobStateCREATED:                 {StepByStepMajor, true},
	JobStateCONSOLIDATIONINPROGRESS: {StepByStepCritical, false},
	JobStateCONSOLIDATIONDONE:       {StepByStepMajor, true},
	JobStateCONSOLIDATIONINDEXED:    {StepByStepAll, true},
	JobStateCONSOLIDATIONEFFECTIVE:  {StepByStepCritical, true},
	JobStateCONSOLIDATIONFAILED:     {StepByStepAll, false},
	JobStateCONSOLIDATIONRETRYING:   {StepByStepMajor, true},
	JobStateCONSOLIDATIONCANCELLING: {StepByStepMajor, true},
	JobStateDELETIONINPROGRESS:      {StepByStepCritical, true},
	JobStateDELETIONEFFECTIVE:       {StepByStepMajor, true},
	JobStateDELETIONFAILED:          {StepByStepAll, false},
	JobStateDONE:                    {StepByStepNever, false},
	JobStateFAILED:                  {StepByStepNever, false},
	JobStateINITIALISATIONFAILED:    {StepByStepAll, false},
	JobStateCANCELLATIONFAILED:      {StepByStepAll, false},
	JobStateABORTED:                 {StepByStepCritical, true},
	JobStateROLLBACKFAILED:          {StepByStepAll, false},
	JobStateDONEBUTUNTIDY:           {StepByStepNever, false},
}

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

type ExecutionLevel int32

// ExecutionLevel
const (
	ExecutionSynchronous  ExecutionLevel = iota // Job is done synchronously
	ExecutionAsynchronous                       // Job is done asynchronously, but without any pause
	StepByStepCritical                          // Job is done asynchronously, step-by-step, pausing at every critical steps
	StepByStepMajor                             // Job is done asynchronously, step-by-step, pausing at every major steps
	StepByStepAll                               // Job is done asynchronously, step-by-step, pausing at every steps

	StepByStepNever // For a JobState to never be paused
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
	ExecutionLevel ExecutionLevel
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
func NewConsolidationJob(jobName, layout, instanceID string, executionLevel ExecutionLevel) (*Job, error) {
	id := uuid.New().String()
	if executionLevel == ExecutionSynchronous {
		return nil, NewValidationError("a consolidation job cannot be executed synchronously")
	}
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

		ExecutionLevel: executionLevel,
		Waiting:        false,
	}
	return j, nil
}

// NewDeletionJob creates a new Job to delete datasets and containers
func NewDeletionJob(jobName string, executionLevel ExecutionLevel) *Job {
	id := uuid.New().String()
	j := &Job{
		persistenceState: persistenceStateNEW,
		ID:               id,
		Name:             jobName,
		Type:             JobTypeDELETION,
		CreationTime:     time.Now(),
		LastUpdateTime:   time.Now(),
		ActiveTasks:      0,
		FailedTasks:      0,
		Payload:          JobPayload{},
		Logs: JobLogs{{
			Severity: "INFO",
			Msg:      "Create Job Deletion",
			Status:   JobStateNEW.String(),
			Date:     time.Now(),
		}},

		ExecutionLevel: executionLevel,
		Waiting:        false,
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
func (j *Job) ToProtobuf(logPage, logLimit int) (*pb.Job, error) {
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
		ExecutionLevel: pb.ExecutionLevel(j.ExecutionLevel),
		Waiting:        j.Waiting,
		Logs:           j.Logs.toSliceString(logPage, logLimit),
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
	if err == "" {
		return
	}
	j.Logs = append(j.Logs, JobLog{
		Severity: ERROR,
		Msg:      err,
		Status:   j.State.String(),
		Date:     time.Now(),
	})
	j.dirty()
}

/***************************************************/
/**                  TRIGGERS                     **/
/***************************************************/

// Trigger handles the event and change the state of the job
// Only returns UnhandledEvent
func (j *Job) Trigger(evt JobEvent) error {
	handled := false

	if evt.Status == Continue {
		if j.Waiting {
			j.Waiting = false
			j.dirty()
			handled = true
		}
	} else if evt.Status == RetryForced && jobStateInfo[j.State].RetryForced {
		j.LogMsg(INFO, "Retried by user")
		handled = true
	} else {
		if evt.Error != "" {
			j.LogErr(evt.Error)
		}

		switch j.Type {
		case JobTypeCONSOLIDATION:
			handled = j.triggerConsolidation(evt)
		case JobTypeDELETION:
			handled = j.triggerDeletion(evt)
		case JobTypeINGESTION:
			handled = j.triggerIngestion(evt)
		}
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
	switch j.State {
	case JobStateNEW:
		switch evt.Status {
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
		case PrepareOrdersFailed:
			return j.changeState(JobStateINITIALISATIONFAILED)
		case OrdersPrepared:
			return j.changeState(JobStateCONSOLIDATIONINPROGRESS)
		}
	case JobStateCONSOLIDATIONINPROGRESS:
		switch evt.Status {
		case RetryForced:
			return j.changeState(JobStateCONSOLIDATIONRETRYING)
		case CancelledByUser, CancelledByUserForced:
			j.LogMsg(INFO, "Cancelled by user")
			return j.changeState(JobStateCONSOLIDATIONCANCELLING)
		case ConsolidationFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case SendOrdersFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case ConsolidationDone:
			return j.changeState(JobStateCONSOLIDATIONDONE)
		}
	case JobStateCONSOLIDATIONDONE:
		switch evt.Status {
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case ConsolidationIndexingFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case ConsolidationIndexed:
			return j.changeState(JobStateCONSOLIDATIONINDEXED)
		}
	case JobStateCONSOLIDATIONINDEXED:
		switch evt.Status {
		case CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case SwapDatasetsFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case DatasetsSwapped:
			return j.changeState(JobStateCONSOLIDATIONEFFECTIVE)
		}
	case JobStateCONSOLIDATIONEFFECTIVE:
		switch evt.Status {
		case StartDeletionFailed:
			return j.changeState(JobStateDONEBUTUNTIDY)
		case DeletionStarted:
			return j.changeState(JobStateDONE)
		}
	case JobStateDONE:
		return false

	case JobStateDONEBUTUNTIDY:
		switch evt.Status {
		case RetryForced:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateCONSOLIDATIONEFFECTIVE)
		}

	case JobStateCONSOLIDATIONCANCELLING:
		switch evt.Status {
		case CancellationFailed:
			return j.changeState(JobStateCANCELLATIONFAILED)
		case CancellationDone:
			return j.changeState(JobStateABORTED)
		}
	case JobStateCANCELLATIONFAILED:
		switch evt.Status {
		case Retried, RetryForced:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateCONSOLIDATIONCANCELLING)
		}
	case JobStateINITIALISATIONFAILED:
		switch evt.Status {
		case Retried, RetryForced:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateCREATED)
		case CancelledByUser, CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		}
	case JobStateCONSOLIDATIONFAILED:
		switch evt.Status {
		case Retried, RetryForced:
			return j.changeState(JobStateCONSOLIDATIONRETRYING)
		case CancelledByUser, CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		}
	case JobStateABORTED:
		switch evt.Status {
		case RollbackFailed:
			return j.changeState(JobStateROLLBACKFAILED)
		case RollbackDone:
			return j.changeState(JobStateFAILED)
		}
	case JobStateROLLBACKFAILED:
		switch evt.Status {
		case RetryForced, Retried:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateABORTED)
		case CancelledByUserForced:
			return j.changeState(JobStateFAILED)
		}

	case JobStateCONSOLIDATIONRETRYING:
		switch evt.Status {
		case ConsolidationRetryFailed:
			return j.changeState(JobStateCONSOLIDATIONFAILED)
		case OrdersPrepared:
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
	switch j.State {
	case JobStateNEW:
		switch evt.Status {
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
		case DeletionNotReady:
			return j.changeState(JobStateINITIALISATIONFAILED)
		case DeletionReady:
			return j.changeState(JobStateDELETIONINPROGRESS)
		}
	case JobStateDELETIONINPROGRESS:
		switch evt.Status {
		case CancelledByUser:
			if j.Waiting {
				return j.changeState(JobStateABORTED)
			}
		case RemovalFailed:
			return j.changeState(JobStateDELETIONFAILED)
		case RemovalDone:
			return j.changeState(JobStateDELETIONEFFECTIVE)
		}
	case JobStateDELETIONEFFECTIVE:
		switch evt.Status {
		case DeletionFailed:
			return j.changeState(JobStateDONEBUTUNTIDY)
		case DeletionDone:
			return j.changeState(JobStateDONE)
		}
	case JobStateDONE:
		return false

	case JobStateDONEBUTUNTIDY:
		switch evt.Status {
		case RetryForced:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateDELETIONEFFECTIVE)
		}
	case JobStateDELETIONFAILED:
		switch evt.Status {
		case Retried, RetryForced:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateDELETIONINPROGRESS)
		case CancelledByUser, CancelledByUserForced:
			return j.changeState(JobStateABORTED)
		}

	case JobStateABORTED:
		switch evt.Status {
		case RollbackFailed:
			return j.changeState(JobStateROLLBACKFAILED)
		case RollbackDone:
			return j.changeState(JobStateFAILED)
		}
	case JobStateROLLBACKFAILED:
		switch evt.Status {
		case RetryForced, Retried:
			j.LogMsg(INFO, "Retried by user")
			return j.changeState(JobStateABORTED)
		case CancelledByUserForced:
			return j.changeState(JobStateFAILED)
		}

	case JobStateFAILED:
		return false
	default:
		panic("trigger: Unknown state")
	}

	return false
}

func (j *Job) triggerIngestion(evt JobEvent) bool {
	panic("TODO Ingestion Not Implemented")
}

func (j *Job) changeState(newState JobState) bool {
	j.State = newState
	j.Waiting = j.ExecutionLevel >= jobStateInfo[j.State].Level
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

// CreateDeletionTask adds a new deletion task with the container uri provided
func (j *Job) CreateDeletionTask(containerURI string) error {
	t, err := newDeletionTask(containerURI)
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
	if j.State != JobStateCONSOLIDATIONINPROGRESS && j.State != JobStateDELETIONEFFECTIVE && j.State != JobStateCONSOLIDATIONCANCELLING {
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

func (jl JobLogs) toSliceString(page, limit int) []string {
	sort.Sort(jl)
	var result []string
	last := utils.MaxI(0, len(jl)-page*limit)
	first := utils.MaxI(0, len(jl)-(page+1)*limit)
	if first != 0 {
		result = append(result, fmt.Sprintf("[... %d more]", first))
	}
	for i := first; i < last; i++ {
		result = append(result, fmt.Sprintf("%s - %s | Status: %s - Message: %s", jl[i].Date.Format(time.RFC3339Nano), jl[i].Severity, jl[i].Status, jl[i].Msg))
	}
	if last != len(jl) {
		result = append(result, fmt.Sprintf("[... %d more]", len(jl)-last))
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
// Only the new ids are available (those who are already locked are not available)
func (l LockedDatasets) NewIDs() []string {
	return l.newDatasetsID.Slice()
}
