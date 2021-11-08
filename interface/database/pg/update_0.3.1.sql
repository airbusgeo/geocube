-- Commit: database: add indexes
DROP INDEX geocube.record_name_idx;
DROP INDEX geocube.definition_name_idx;
DROP INDEX geocube.ix_geocube_locked_datasets_job_id;
CREATE INDEX idx_records_aoi ON geocube.records (aoi_id);
CREATE INDEX idx_instance_definition ON geocube.variable_instances (definition_id);
CREATE INDEX idx_datasets_container ON geocube.datasets (container_uri);
CREATE INDEX idx_datasets_record ON geocube.datasets (record_id);
CREATE INDEX idx_datasets_instance ON geocube.datasets (instance_id);
CREATE INDEX idx_locked_datasets_job_id ON geocube.locked_datasets (job_id);
CREATE INDEX idx_tasks_job ON geocube.tasks (job_id);

-- Commit: step-by-step jobs
ALTER TABLE geocube.jobs ADD COLUMN step_by_step INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE geocube.jobs ADD COLUMN waiting BOOLEAN DEFAULT FALSE NOT NULL;

-- Commit: Add DeleteLayout, remove layout.id
ALTER TABLE geocube.layouts DROP COLUMN id;

-- Commit: ConsolidationParams: downsampling=>resampling
ALTER TABLE geocube.consolidation_params RENAME COLUMN downsampling_alg TO resampling_alg;
