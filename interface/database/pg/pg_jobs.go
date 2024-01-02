package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/airbusgeo/geocube/interface/database"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/lib/pq"
)

const (
	logsSubtable = `
		(SELECT jobs.*, json_agg(json_build_object('time', to_char((time::timestamp), 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), 'severity', severity,'status', status, 'message', message)) AS logs
		FROM geocube.jobs
		LEFT JOIN LATERAL (
			SELECT * FROM geocube.job_logs
			WHERE job_logs.job_id = jobs.id
			ORDER BY job_logs.time DESC
			OFFSET %d LIMIT %d
		) sub ON TRUE
		GROUP BY jobs.id) jobs
	`
)

func reverse(log geocube.JobLogs) {
	for i, j := 0, len(log)-1; i < j; i, j = i+1, j-1 {
		log[i], log[j] = log[j], log[i]
	}
}

// FindJobs implements GeocubeBackend
func (b Backend) FindJobs(ctx context.Context, nameLike string, page, limit int) ([]*geocube.Job, error) {
	wc := joinClause{}
	if nameLike != "" {
		nameLike, operator := parseLike(nameLike)
		wc.append(" name "+operator+" $%d", nameLike)
	}

	rows, err := b.pg.QueryContext(ctx,
		"SELECT id, name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, execution_level, waiting, logs"+
			" FROM "+fmt.Sprintf(logsSubtable, 0, 10)+
			wc.WhereClause()+limitOffsetClause(page, limit), wc.Parameters...)

	if err != nil {
		return nil, pqErrorFormat("FindJobs: %w", err)
	}
	defer rows.Close()

	jobs := []*geocube.Job{}
	for rows.Next() {
		j := geocube.Job{LogsCount: -1}
		if err := rows.Scan(&j.ID, &j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.ExecutionLevel, &j.Waiting, &j.Logs); err != nil {
			return nil, fmt.Errorf("FindJobs: %w", err)
		}
		reverse(j.Logs)
		jobs = append(jobs, &j)
	}
	return jobs, nil
}

// ReadJob implements GeocubeBackend
func (b Backend) ReadJob(ctx context.Context, jobID string, opts ...database.ReadJobOptions) (*geocube.Job, error) {
	var err error

	readJobOpts := database.Apply(opts...)

	j := geocube.NewJob(jobID)

	if readJobOpts.Limit == 0 {
		err = b.pg.QueryRowContext(ctx,
			"SELECT name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, execution_level, waiting"+
				" FROM geocube.jobs WHERE id = $1", jobID).
			Scan(&j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.ExecutionLevel, &j.Waiting)
	} else {
		err = b.pg.QueryRowContext(ctx,
			"SELECT name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, execution_level, waiting, logs, log_count.*"+
				" FROM "+fmt.Sprintf(logsSubtable, readJobOpts.Page*readJobOpts.Limit, readJobOpts.Limit)+
				" LEFT JOIN LATERAL (SELECT count(*) from geocube.job_logs WHERE job_logs.job_id = jobs.id) log_count on true"+
				" WHERE id = $1", jobID).
			Scan(&j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.ExecutionLevel, &j.Waiting, &j.Logs, &j.LogsCount)
	}

	switch {
	case err == sql.ErrNoRows:
		// Job has not been found
		return nil, geocube.NewEntityNotFound("Job", "id", jobID, "")

	case err != nil:
		return nil, pqErrorFormat("ReadJob.QueryRowContext: %w", err)
	}

	reverse(j.Logs)
	return j, nil
}

// ReadJobWithTask implements GeocubeBackend
func (b Backend) ReadJobWithTask(ctx context.Context, jobID string, taskID string) (*geocube.Job, error) {
	j := geocube.NewJob(jobID)

	var t geocube.Task
	err := b.pg.QueryRowContext(ctx,
		"SELECT j.name, j.type, j.creation_ts, j.last_update_ts, j.state, j.active_tasks, j.failed_tasks, j.payload, j.execution_level, j.waiting, t.id, t.state "+
			"FROM geocube.jobs j JOIN geocube.tasks t ON j.id = t.job_id WHERE j.id = $1 AND t.id = $2", jobID, taskID).
		Scan(&j.Name, &j.Type, &j.CreationTime, &j.LastUpdateTime, &j.State, &j.ActiveTasks, &j.FailedTasks, &j.Payload, &j.ExecutionLevel, &j.Waiting, &t.ID, &t.State)

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
		"INSERT INTO geocube.jobs (id, name, type, creation_ts, last_update_ts, state, active_tasks, failed_tasks, payload, execution_level)"+
			" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		job.ID, job.Name, job.Type, job.CreationTime, job.LastUpdateTime, job.State, job.ActiveTasks, job.FailedTasks, job.Payload, job.ExecutionLevel)

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
		"UPDATE geocube.jobs SET last_update_ts = $1, state = $2, active_tasks = $3, failed_tasks = $4, waiting = $5"+
			" WHERE id = $6 AND last_update_ts = $7",
		job.LastUpdateTime, job.State, job.ActiveTasks, job.FailedTasks, job.Waiting, job.ID, job.OCCTime())

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

func (b Backend) PersistLogs(ctx context.Context, jobID string, logs geocube.JobLogs) error {
	if len(logs) == 0 {
		return nil
	}

	// Prepare the insert
	stmt, err := b.pg.PrepareContext(ctx, pq.CopyInSchema("geocube", "job_logs", "job_id", "time", "status", "message", "severity"))
	if err != nil {
		return pqErrorFormat("PersistLogs.prepare: %w", err)
	}
	defer func() {
		if e := stmt.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Append logs
	for _, log := range logs {
		if _, err = stmt.ExecContext(ctx, jobID, log.Date, log.Status, log.Msg, log.Severity); err != nil {
			return pqErrorFormat("PersistLogs.append.exec: %w", err)
		}
	}

	// Execute statement
	if _, err = stmt.ExecContext(ctx); err != nil {
		return pqErrorFormat("PersistLogs.exec: %w", err)
	}

	return nil
}

// ListJobsID implements GeocubeBackend
func (b Backend) ListJobsID(ctx context.Context, nameLike string, states []geocube.JobState) ([]string, error) {
	wc := joinClause{}

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

	// Update datasets table
	_, err = b.pg.ExecContext(ctx, "UPDATE geocube.datasets SET locked_by_job_id=$1 WHERE id = ANY($2)", lockedByJobID, pq.Array(datasetsID))
	switch pqErrorCode(err) {
	case noError:
		return nil
	default:
		return pqErrorFormat("ReleaseDatasets: %w", err)
	}
}

// ReleaseDatasets implements GeocubeBackend
func (b Backend) ReleaseDatasets(ctx context.Context, lockedByJobID string, flag int) error {

	// Update datasets table
	_, err := b.pg.ExecContext(ctx,
		"UPDATE geocube.datasets d SET locked_by_job_id=NULL FROM geocube.locked_datasets l"+
			" WHERE d.locked_by_job_id = $1 AND d.id = l.dataset_id AND l.flag = $2", lockedByJobID, flag)
	switch pqErrorCode(err) {
	case noError:
	default:
		return pqErrorFormat("ReleaseDatasets: %w", err)
	}

	// Release Datasets
	_, err = b.pg.ExecContext(ctx, "DELETE FROM geocube.locked_datasets WHERE job_id = $1 AND flag = $2", lockedByJobID, flag)
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
		"UPDATE geocube.datasets SET status = $1 WHERE locked_by_job_id = $2 AND status = $3", toStatus, lockedByJobID, fromStatus)
	if err != nil {
		return pqErrorFormat(fmt.Sprintf("ChangeDatasetsStatus[%s] %s->%s: %%w", lockedByJobID, fromStatus.String(), toStatus.String()), err)
	}
	return nil
}

// CreateConsolidationParams implements GeocubeBackend
func (b Backend) CreateConsolidationParams(ctx context.Context, id string, cp geocube.ConsolidationParams) error {
	_, err := b.pg.ExecContext(ctx,
		"INSERT INTO geocube.consolidation_params (id, dtype, no_data, min_value, max_value, exponent,"+
			"compression, creation_params, resampling_alg, storage_class) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"+
			" ON CONFLICT (id) DO UPDATE"+
			" SET dtype=EXCLUDED.dtype, no_data=EXCLUDED.no_data, min_value=EXCLUDED.min_value, max_value=EXCLUDED.max_value, exponent=EXCLUDED.exponent,"+
			" compression=EXCLUDED.compression, creation_params=EXCLUDED.creation_params, resampling_alg=EXCLUDED.resampling_alg,"+
			" storage_class=EXCLUDED.storage_class",
		id, cp.DFormat.DType, cp.DFormat.NoData, cp.DFormat.Range.Min, cp.DFormat.Range.Max, cp.Exponent,
		cp.Compression, cp.CreationParams, cp.ResamplingAlg, cp.StorageClass)

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
	err := b.pg.QueryRowContext(ctx, "SELECT dtype, no_data, min_value, max_value, compression, creation_params, resampling_alg, exponent, storage_class"+
		" FROM geocube.consolidation_params WHERE id = $1", id).
		Scan(&cp.DFormat.DType, &cp.DFormat.NoData, &cp.DFormat.Range.Min, &cp.DFormat.Range.Max,
			&cp.Compression, &cp.CreationParams, &cp.ResamplingAlg, &cp.Exponent, &cp.StorageClass)
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
