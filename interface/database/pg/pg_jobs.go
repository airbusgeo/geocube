package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/lib/pq"
)

// FindJobs implements GeocubeBackend
func (b Backend) FindJobs(ctx context.Context, nameLike string) ([]*geocube.Job, error) {
	wc := whereClause{}
	if nameLike != "" {
		nameLike, operator := parseLike(nameLike)
		wc.append(" name "+operator+" $%d", nameLike)
	}
	rows, err := b.pg.QueryContext(ctx,
		"SELECT id, name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, step_by_step, waiting FROM geocube.jobs"+wc.WhereClause(), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("FindJobs: %w", err)
	}
	defer rows.Close()

	jobs := []*geocube.Job{}
	for rows.Next() {
		var j geocube.Job
		if err := rows.Scan(&j.ID, &j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.StepByStep, &j.Waiting); err != nil {
			return nil, fmt.Errorf("FindJobs: %w", err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}

// ReadJob implements GeocubeBackend
func (b Backend) ReadJob(ctx context.Context, jobID string) (*geocube.Job, error) {
	j := geocube.NewJob(jobID)
	err := b.pg.QueryRowContext(ctx,
		"SELECT name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, step_by_step, waiting FROM geocube.jobs WHERE id = $1", jobID).
		Scan(&j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.StepByStep, &j.Waiting)

	switch {
	case err == sql.ErrNoRows:
		// Job has not been found
		return nil, geocube.NewEntityNotFound("Job", "id", jobID, "")

	case err != nil:
		return nil, pqErrorFormat("ReadJob.QueryRowContext: %w", err)
	}

	return j, nil
}

// ReadJobWithTask implements GeocubeBackend
func (b Backend) ReadJobWithTask(ctx context.Context, jobID string, taskID string) (*geocube.Job, error) {
	j := geocube.NewJob(jobID)

	var t geocube.Task
	err := b.pg.QueryRowContext(ctx,
		"SELECT j.name, j.type, j.creation_ts, j.last_update_ts, j.state, j.active_tasks, j.failed_tasks, j.payload, j.step_by_step, j.waiting, t.id, t.state "+
			"FROM geocube.jobs j JOIN geocube.tasks t ON j.id = t.job_id WHERE j.id = $1 AND t.id = $2", jobID, taskID).
		Scan(&j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.StepByStep, &j.Waiting, &t.ID, &t.State)

	switch {
	case err == sql.ErrNoRows:
		// Job has not been found
		return nil, geocube.NewEntityNotFound("Job/Task", "id/id", jobID+"/"+taskID, "")

	case err != nil:
		return nil, pqErrorFormat("ReadJobWithTask.QueryRowContext: %w", err)
	}

	j.Tasks = []*geocube.Task{&t}
	return j, nil
}

// CreateJob implements GeocubeBackend
func (b Backend) CreateJob(ctx context.Context, job *geocube.Job) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.jobs (id, name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, step_by_step)"+
			" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		job.ID, job.Name, job.Type, job.CreationTime, job.LastUpdateTime, job.State, job.ActiveTasks, job.FailedTasks, job.Payload, job.StepByStep)

	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		return geocube.NewEntityAlreadyExists("Job", "name", job.Name, "")
	default:
		return pqErrorFormat("CreateJob.exec: %w", err)
	}

	return nil
}

// DeleteJob implements GeocubeBackend
func (b Backend) DeleteJob(ctx context.Context, jobID string) error {
	return b.delete(ctx, "jobs", "id", jobID)
}

// UpdateJob implements GeocubeBackend
func (b Backend) UpdateJob(ctx context.Context, job *geocube.Job) error {
	res, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.jobs SET last_update_ts = $1, state = $2, active_tasks = $3, failed_tasks = $4, payload = jsonb_set(cast(payload as jsonb), '{error}', to_jsonb($5::text), true), waiting = $6"+
			" WHERE id = $7 AND last_update_ts = $8",
		job.LastUpdateTime, job.State, job.ActiveTasks, job.FailedTasks, job.Payload.Err, job.Waiting, job.ID, job.OCCTime())

	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityNotFound("Job", "id/timestamp", job.ID+"/"+job.OCCTime().String(), "")
		}
		return nil
	default:
		return pqErrorFormat("UpdateJob.exec: %w", err)
	}
}

// ListJobsID implements GeocubeBackend
func (b Backend) ListJobsID(ctx context.Context, nameLike string, states []geocube.JobState) ([]string, error) {
	wc := whereClause{}

	strStates := make([]string, len(states))
	for i, state := range states {
		strStates[i] = state.String()
	}
	wc.append("state = ANY($%d)", pq.Array(strStates))

	if nameLike != "" {
		nameLike, operator := parseLike(nameLike)
		wc.append("name "+operator+" $%d", nameLike)
	}

	rows, err := b.pg.QueryContext(ctx, "SELECT id FROM geocube.jobs"+wc.WhereClause(), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("ListJobsID: %w", err)
	}

	return scanIdsAndClose(rows)
}

// LockDatasets implements GeocubeBackend
func (b Backend) LockDatasets(ctx context.Context, lockedByJobID string, datasetsID []string, flag int) (err error) {
	// Prepare the insert
	stmt, err := b.pg.PrepareContext(ctx, pq.CopyInSchema("geocube", "locked_datasets", "dataset_id", "job_id", "flag"))
	if err != nil {
		return pqErrorFormat("LockDatasets.prepare: %w", err)
	}
	defer func() {
		if e := stmt.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Append the datasets
	for _, datasetID := range datasetsID {
		if _, err = stmt.ExecContext(ctx, datasetID, lockedByJobID, flag); err != nil {
			return pqErrorFormat("LockDatasets.exec1: %w", err)
		}
	}

	// Execute statement
	_, err = stmt.ExecContext(ctx)
	switch pqErrorCode(err) {
	case noError:
	case uniqueViolation:
		// TODO: what is the error when only some datasets fail ? Should we try again with the good datasets only ?
		if _, id := extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
			return geocube.NewEntityAlreadyExists("dataset", "id", id, "Dataset is already locked: "+id)
		}
		return geocube.NewEntityAlreadyExists("", "", "", "One of the datasets is already locked")
	case foreignKeyViolation:
		return pqErrorFormat("LockDatasets.exec2: %w", err)
	default:
		return pqErrorFormat("LockDatasets.exec2: %w", err)
	}

	return nil
}

// ReleaseDatasets implements GeocubeBackend
func (b Backend) ReleaseDatasets(ctx context.Context, lockedByJobID string, flag int) error {
	_, err := b.pg.ExecContext(ctx, "DELETE FROM geocube.locked_datasets WHERE job_id = $1 AND flag = $2", lockedByJobID, flag)
	switch pqErrorCode(err) {
	case noError:
		return nil
	default:
		return pqErrorFormat("ReleaseDatasets: %w", err)
	}
}

// CreateTasks implements GeocubeBackend
func (b Backend) CreateTasks(ctx context.Context, jobID string, tasks []*geocube.Task) error {
	data := make([][]interface{}, len(tasks))
	for i, task := range tasks {
		data[i] = []interface{}{task.ID, task.State, task.Payload, jobID}
	}

	err := b.bulkInsert(ctx, "geocube", "tasks", []string{"id", "state", "payload", "job_id"}, data)

	switch pqErrorCode(err) {
	case noError:
		return nil
	case uniqueViolation:
		// TODO: what is the error when only some tasks fail ? Should we try again with the good tasks only ?
		if key, id := extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
			return geocube.NewEntityAlreadyExists("Task", key, id, "")
		}
		return geocube.NewEntityAlreadyExists("", "", "", "Tasks")
	default:
		return pqErrorFormat("CreateTasks: %w", err)
	}
}

// ReadTasks implements GeocubeBackend
func (b Backend) ReadTasks(ctx context.Context, jobID string, states []geocube.TaskState) (tasks []*geocube.Task, err error) {
	var rows *sql.Rows

	if states == nil {
		rows, err = b.pg.QueryContext(ctx, "SELECT id, state, payload FROM geocube.tasks WHERE job_id=$1", jobID)
	} else {
		strStates := make([]string, len(states))
		for i, s := range states {
			strStates[i] = s.String()
		}
		rows, err = b.pg.QueryContext(ctx, "SELECT id, state, payload FROM geocube.tasks WHERE job_id=$1 and state=ANY($2)", jobID, pq.Array(strStates))
	}

	if err != nil {
		return nil, pqErrorFormat("ReadTasks.Query: %w", err)
	}
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	for rows.Next() {
		var task geocube.Task
		err := rows.Scan(&task.ID, &task.State, &task.Payload)
		if err != nil {
			return nil, pqErrorFormat("ReadTasks.Scan: %w", err)
		}
		tasks = append(tasks, &task)
	}
	return tasks, nil
}

// UpdateTask implements GeocubeBackend
func (b Backend) UpdateTask(ctx context.Context, task *geocube.Task) error {
	res, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.tasks SET state = $1 WHERE id = $2", task.State, task.ID)

	switch pqErrorCode(err) {
	case noError:
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return geocube.NewEntityNotFound("Task", "id", task.ID, "")
		}
		return nil
	default:
		return pqErrorFormat("UpdateTask: %w", err)
	}
}

// DeleteTask implements GeocubeBackend
func (b Backend) DeleteTask(ctx context.Context, taskID string) error {
	return b.delete(ctx, "tasks", "id", taskID)
}

// ChangeDatasetsStatus implements GeocubeBackend
func (b Backend) ChangeDatasetsStatus(ctx context.Context, lockedByJobID string, fromStatus geocube.DatasetStatus, toStatus geocube.DatasetStatus) error {
	_, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.datasets d SET status = $1 FROM geocube.locked_datasets l "+
			"WHERE l.job_id = $2 AND l.dataset_id = d.id AND d.status = $3", toStatus, lockedByJobID, fromStatus)
	if err != nil {
		return pqErrorFormat(fmt.Sprintf("ChangeDatasetsStatus[%s] %s->%s: %%w", lockedByJobID, fromStatus.String(), toStatus.String()), err)
	}
	return nil
}

// CreateConsolidationParams implements GeocubeBackend
func (b Backend) CreateConsolidationParams(ctx context.Context, id string, cp geocube.ConsolidationParams) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.consolidation_params (id, dtype, no_data, min_value, max_value, exponent,"+
			"compression, overviews, downsampling_alg, bands_interleave, storage_class) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)"+
			" ON CONFLICT (id) DO UPDATE"+
			" SET dtype=EXCLUDED.dtype, no_data=EXCLUDED.no_data, min_value=EXCLUDED.min_value, max_value=EXCLUDED.max_value, exponent=EXCLUDED.exponent,"+
			" compression=EXCLUDED.compression, overviews=EXCLUDED.overviews, downsampling_alg=EXCLUDED.downsampling_alg,"+
			" bands_interleave=EXCLUDED.bands_interleave, storage_class=EXCLUDED.storage_class",
		id, cp.DFormat.DType, cp.DFormat.NoData, cp.DFormat.Range.Min, cp.DFormat.Range.Max, cp.Exponent,
		cp.Compression, cp.Overviews, cp.DownsamplingAlg, cp.BandsInterleave, cp.StorageClass)

	switch pqErrorCode(err) {
	case noError:
		return nil
	default:
		return pqErrorFormat("CreateConsolidationParams: %w", err)
	}
}

// ReadConsolidationParams implements geocubeBackend
func (b Backend) ReadConsolidationParams(ctx context.Context, id string) (*geocube.ConsolidationParams, error) {
	var cp geocube.ConsolidationParams
	err := b.pg.QueryRowContext(ctx, "SELECT dtype, no_data, min_value, max_value, compression, overviews, downsampling_alg, exponent, bands_interleave, storage_class"+
		" FROM geocube.consolidation_params WHERE id = $1", id).
		Scan(&cp.DFormat.DType, &cp.DFormat.NoData, &cp.DFormat.Range.Min, &cp.DFormat.Range.Max,
			&cp.Compression, &cp.Overviews, &cp.DownsamplingAlg, &cp.Exponent, &cp.BandsInterleave, &cp.StorageClass)
	switch {
	case err == sql.ErrNoRows:
		return nil, geocube.NewEntityNotFound("ConsolidationParams", "id", id, "")
	case err != nil:
		return nil, pqErrorFormat("ReadConsolidationParams: %w", err)
	}
	return &cp, nil
}

// DeleteConsolidationParams implements geocubeBackend
func (b Backend) DeleteConsolidationParams(ctx context.Context, id string) error {
	return b.delete(ctx, "consolidation_params", "id", id)
}

// DeletePendingConsolidationParams implements GeocubeBackend
func (b Backend) DeletePendingConsolidationParams(ctx context.Context) (int64, error) {
	// Delete consolidation params
	res, err := b.pg.ExecContext(ctx, "DELETE from geocube.consolidation_params p"+
		" WHERE NOT EXISTS (SELECT NULL FROM geocube.variable_definitions d WHERE p.id = d.id)"+
		" AND NOT EXISTS (SELECT NULL FROM geocube.jobs j WHERE p.id = j.id)")

	if err != nil {
		return 0, pqErrorFormat("DeletePendingConsolidationParams: %w", err)
	}

	return res.RowsAffected()
}
