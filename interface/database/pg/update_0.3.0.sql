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
ALTER TABLE geocube.jobs ADD COLUMN execution_level INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE geocube.jobs ADD COLUMN waiting BOOLEAN DEFAULT FALSE NOT NULL;

-- Commit: Add DeleteLayout, remove layout.id
ALTER TABLE geocube.layouts DROP COLUMN id;

-- Commit: ConsolidationParams: downsampling=>resampling
ALTER TABLE geocube.consolidation_params RENAME COLUMN downsampling_alg TO resampling_alg;

-- update geometry type
ALTER TABLE geocube.datasets ALTER COLUMN geog TYPE geography(MULTIPOLYGON,0) USING ST_Multi(geog::geometry)::geography;
ALTER TABLE geocube.datasets ALTER COLUMN geom TYPE geometry(MULTIPOLYGON,4326) USING ST_MULTI(geom);
ALTER TABLE geocube.datasets ALTER COLUMN shape TYPE geometry(MULTIPOLYGON,0) USING ST_MULTI(shape);

-- add message column on job table
ALTER TABLE geocube.jobs ADD COLUMN logs JSONB default '[]'::JSONB;

-- add container_layouts
CREATE TABLE geocube.container_layouts (
	container_uri TEXT NOT NULL,
	layout_name TEXT NOT NULL,
	PRIMARY KEY (container_uri),
	FOREIGN KEY(container_uri) REFERENCES geocube.containers (uri) MATCH FULL ON DELETE CASCADE ON UPDATE CASCADE,
 	FOREIGN KEY(layout_name) REFERENCES geocube.layouts (name) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
);

-- Commit: CustomGrid for layout
CREATE TABLE geocube.grids (
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	PRIMARY KEY (name)
);

CREATE TABLE geocube.cells (
	id TEXT NOT NULL,
	grid TEXT NOT NULL,
	crs TEXT NOT NULL,
	srid INTEGER NOT NULL,
	coordinates geography(POLYGON,0),
	PRIMARY KEY (id, grid),
	FOREIGN KEY(grid) REFERENCES geocube.grids (name) MATCH FULL ON DELETE NO ACTION ON UPDATE NO ACTION
);
CREATE INDEX idx_cells_coordinates ON geocube.cells USING GIST (coordinates);
CREATE INDEX idx_cells_grid ON geocube.cells (grid);

-- create_overviews => overviews_min_size
ALTER TABLE geocube.consolidation_params ADD COLUMN overviews_min_size INTEGER default -1;
UPDATE geocube.consolidation_params SET overviews_min_size=0 WHERE overviews=FALSE;
ALTER TABLE geocube.consolidation_params DROP COLUMN overviews;