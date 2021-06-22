package geocube

import (
	"bytes"
	"fmt"

	"github.com/google/uuid"
)

//go:generate enumer -json -sql -type TaskState -trimprefix TaskState

type TaskState int32

const (
	// TaskStateFAILED when the execution encountered an error
	TaskStateFAILED TaskState = iota
	// TaskStateCANCELLED when the task is cancelled by the server or there is no operation to perform (consolidation task: no container is created)
	TaskStateCANCELLED
	// TaskStateDONE when is the task is finished and successful
	TaskStateDONE
	// TaskStatePENDING when the task is waiting for a new status
	TaskStatePENDING
)

type Task struct {
	persistenceState
	ID      string
	State   TaskState
	Payload []byte
}

// newConsolidationTask creates a new task with the consolidation event provided
func newConsolidationTask(evt ConsolidationEvent) (*Task, error) {
	// Create a new uuid
	evt.TaskID = uuid.New().String()

	// Marshal the payload
	payload, err := MarshalConsolidationEvent(evt)
	if err != nil {
		return nil, fmt.Errorf("newConsolidationTask: %w", err)
	}

	return &Task{
		persistenceState: persistenceStateNEW,
		ID:               evt.TaskID,
		State:            TaskStatePENDING,
		Payload:          payload,
	}, nil
}

// ConsolidationOutput retrieves the output of the consolidation payload
func (t *Task) ConsolidationOutput() (*ConsolidationContainer, []ConsolidationRecord, error) {
	// Unmarshal the payload
	evt, err := UnmarshalConsolidationEvent(bytes.NewReader(t.Payload))
	if err != nil {
		return nil, nil, fmt.Errorf("ConsolidationOutput.%w", err)
	}
	return &evt.Container, evt.Records, nil
}

// setState changes the state of the tasks
// returns true if the state has changed
func (t *Task) setState(newState TaskState) bool {
	if t.State != newState {
		t.State = newState
		t.dirty()
		return true
	}
	return false
}
