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

