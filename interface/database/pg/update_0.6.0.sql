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

-- add interlacing_pattern and remove band_interleave
ALTER TABLE geocube.layouts ADD COLUMN interlacing_pattern TEXT NOT NULL DEFAULT 'Z=0>T>R>B;R>Z=1:>T>B';
ALTER TABLE geocube.consolidation_params DROP COLUMN bands_interleave;

-- consolidation_params.overviews_min_size => layout.overviews_min_size
ALTER TABLE geocube.layouts ADD COLUMN overviews_min_size INTEGER default -1;
ALTER TABLE geocube.consolidation_params DROP COLUMN overviews_min_size;