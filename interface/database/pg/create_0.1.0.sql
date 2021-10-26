CREATE SCHEMA geocube;
CREATE SCHEMA IF NOT EXISTS public;
CREATE EXTENSION IF NOT EXISTS hstore;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE TYPE geocube.datatypes AS ENUM ('BOOL', 'UINT8', 'UINT16', 'INT16', 'UINT32', 'INT32', 'FLOAT32', 'FLOAT64', 'COMPLEX64');
CREATE TYPE geocube.compression AS ENUM ('NO', 'LOSSLESS', 'LOSSY');
CREATE TYPE geocube.resampling AS ENUM ('NEAR', 'BILINEAR', 'CUBIC', 'CUBICSPLINE', 'LANCZOS', 'AVERAGE', 'MODE', 'MAX', 'MIN', 'MED', 'Q1', 'Q3');
CREATE TYPE geocube.dataset_status AS ENUM ('ACTIVE', 'TODELETE', 'INACTIVE');
CREATE TYPE geocube.data_types AS ENUM ('BOOL', 'UINT8', 'UINT16', 'INT16', 'UINT32', 'INT32', 'FLOAT32', 'FLOAT64', 'COMPLEX64');
CREATE TYPE geocube.storage_class AS ENUM ('STANDARD', 'INFREQUENT', 'ARCHIVE', 'DEEPARCHIVE');
CREATE TYPE geocube.task_state AS ENUM ('PENDING', 'DONE', 'FAILED', 'CANCELLED');
CREATE TYPE geocube.color_point AS (
	value real,
	rgba bigint
);


CREATE TABLE geocube.aoi (
	id UUID NOT NULL, 
	hash TEXT, 
	geom geometry(MULTIPOLYGON,4326), 
	PRIMARY KEY (id), 
	UNIQUE (hash)
);
CREATE INDEX idx_aoi_geom ON geocube.aoi USING GIST (geom);

CREATE TABLE geocube.records (
	id UUID NOT NULL,
	name TEXT NOT NULL,
	datetime TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    tags HSTORE NOT NULL,
	aoi_id UUID NOT NULL,
	PRIMARY KEY (id),
    UNIQUE (name, tags, datetime),
	FOREIGN KEY(aoi_id) REFERENCES geocube.aoi (id) MATCH FULL
);
CREATE INDEX idx_records_aoi ON geocube.records (aoi_id);

CREATE TABLE geocube.palette (
	name TEXT NOT NULL,
	points geocube.color_point[] NOT NULL,
	PRIMARY KEY (name)
);

CREATE TABLE geocube.variable_definitions (
	id UUID NOT NULL, 
	name TEXT NOT NULL, 
	unit TEXT NOT NULL,
	description TEXT NOT NULL,
	bands TEXT[],
	dtype geocube.datatypes NOT NULL,
	no_data double precision NOT NULL,
	min_value double precision NOT NULL,
	max_value double precision NOT NULL,
	palette TEXT REFERENCES geocube.palette,
	resampling_alg geocube.resampling NOT NULL,
	PRIMARY KEY (id), 
	UNIQUE (name)
);

CREATE TABLE geocube.variable_instances (
	id UUID NOT NULL,
	name TEXT NOT NULL,
	metadata HSTORE,
	definition_id UUID,
	PRIMARY KEY (id),
	UNIQUE (name, definition_id),
	FOREIGN KEY(definition_id) REFERENCES geocube.variable_definitions (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
);
CREATE INDEX idx_instance_definition ON geocube.variable_instances (definition_id);

CREATE TABLE geocube.containers (
	uri TEXT NOT NULL, 
	managed BOOLEAN NOT NULL,
	storage_class geocube.storage_class,
	PRIMARY KEY (uri)
);

CREATE TABLE geocube.datasets (
	id UUID NOT NULL, 
	record_id UUID NOT NULL, 
	instance_id UUID NOT NULL, 
	container_uri TEXT NOT NULL, 
	geog geography(POLYGON,0) NOT NULL,
	geom geometry(POLYGON,4326) NOT NULL,
	shape geometry(POLYGON,0) NOT NULL,
	subdir TEXT NOT NULL, 
	bands SMALLINT[] NOT NULL, 
	status geocube.dataset_status NOT NULL,
	dtype geocube.data_types NOT NULL,
	no_data double precision NOT NULL,
	min_value double precision NOT NULL,
	max_value double precision NOT NULL,
	real_min_value double precision NOT NULL,
	real_max_value double precision NOT NULL,
	exponent double precision not NULL,
	overviews BOOLEAN NOT NULL,
	PRIMARY KEY (id), 
	FOREIGN KEY(record_id) REFERENCES geocube.records (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION, 
	FOREIGN KEY(instance_id) REFERENCES geocube.variable_instances (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION, 
	FOREIGN KEY(container_uri) REFERENCES geocube.containers (uri) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
);
CREATE INDEX idx_datasets_geog ON geocube.datasets USING GIST (geog);
CREATE INDEX idx_datasets_geom ON geocube.datasets USING GIST (geom);
CREATE INDEX idx_datasets_container ON geocube.datasets (container_uri);
CREATE INDEX idx_datasets_record ON geocube.datasets (record_id);
CREATE INDEX idx_datasets_instance ON geocube.datasets (instance_id);

CREATE TABLE geocube.layouts (
	id UUID NOT NULL,
	name TEXT NOT NULL,
	grid_flags TEXT[],
	grid_parameters HSTORE,
	block_x_size INTEGER DEFAULT '256' NOT NULL,
	block_y_size INTEGER DEFAULT '256' NOT NULL,
	max_records INTEGER DEFAULT '1024' NOT NULL,
	PRIMARY KEY (id),
	UNIQUE (name)
);

CREATE TABLE geocube.jobs (
	id UUID NOT NULL,
	name TEXT NOT NULL,
	creation_ts TIMESTAMP WITHOUT TIME ZONE NOT NULL,
	last_update_ts TIMESTAMP WITHOUT TIME ZONE NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL,
	active_tasks INTEGER NOT NULL,
	failed_tasks INTEGER NOT NULL,
	type TEXT NOT NULL,
	step_by_step INTEGER DEFAULT 0 NOT NULL,
	waiting BOOLEAN DEFAULT FALSE NOT NULL,
	PRIMARY KEY (id),
	UNIQUE (name)
);

CREATE TABLE geocube.consolidation_params (
	id UUID NOT NULL,
	dtype geocube.datatypes NOT NULL,
	no_data double precision NOT NULL,
	min_value double precision NOT NULL,
	max_value double precision NOT NULL,
	exponent double precision NOT NULL,
	compression geocube.compression NOT NULL,
	overviews BOOLEAN NOT NULL,
	downsampling_alg geocube.resampling NOT NULL,
	bands_interleave BOOLEAN NOT NULL,
	storage_class geocube.storage_class NOT NULL,
	PRIMARY KEY (id)
);

CREATE TABLE geocube.locked_datasets (
	dataset_id UUID NOT NULL,
	job_id UUID NOT NULL,
	flag INTEGER NOT NULL,
	PRIMARY KEY (dataset_id),
	FOREIGN KEY(dataset_id) REFERENCES geocube.datasets (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION,
	FOREIGN KEY(job_id) REFERENCES geocube.jobs (id) MATCH FULL ON DELETE CASCADE ON UPDATE NO ACTION
);
CREATE INDEX idx_geocube_locked_datasets_job_id ON geocube.locked_datasets (job_id);

CREATE TABLE geocube.tasks (
	id UUID NOT NULL,
	state geocube.task_state NOT NULL,
	payload bytea NOT NULL,
	job_id UUID NOT NULL,
	PRIMARY KEY (id),
	FOREIGN KEY(job_id) REFERENCES geocube.jobs (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
);
CREATE INDEX idx_tasks_job ON geocube.tasks (job_id);

-- CREATE TABLE geocube.dataset_layouts (
-- 	dataset_id UUID NOT NULL,
-- 	layout_id UUID NOT NULL,
-- 	cell_id TEXT NOT NULL,
-- 	PRIMARY KEY (dataset_id),
-- 	FOREIGN KEY(dataset_id) REFERENCES geocube.datasets (id) MATCH FULL ON DELETE CASCADE ON UPDATE CASCADE,
-- 	FOREIGN KEY(layout_id) REFERENCES geocube.layouts (id) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
-- );


-- CREATE ROLE apiserver WITH LOGIN;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA geocube TO apiserver;
