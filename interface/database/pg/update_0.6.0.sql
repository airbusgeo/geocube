CREATE TYPE geocube.log_level AS ENUM ('INFO', 'DEBUG', 'WARN', 'ERROR');

-- add job_logs
CREATE TABLE geocube.job_logs (
    id SERIAL PRIMARY KEY,
    job_id UUID NOT NULL,
    time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    status TEXT,
    message TEXT,
    severity geocube.log_level NOT NULL,
    FOREIGN KEY(job_id) REFERENCES geocube.jobs (id) MATCH FULL ON DELETE CASCADE ON UPDATE CASCADE
);

-- drop logs column on geocube.jobs
ALTER TABLE geocube.jobs DROP COLUMN logs;

-- add id column on geocube.containers
ALTER TABLE geocube.containers ADD COLUMN id SERIAL;
CREATE INDEX idx_containers_id ON geocube.containers (id);

-- add locked_by_job_id to optimize FindDatasets request by job_id & geog
ALTER TABLE geocube.datasets ADD COLUMN locked_by_job_id UUID;
CREATE INDEX idx_datasets_locked ON geocube.datasets (locked_by_job_id);

