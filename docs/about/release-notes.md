# Release notes

## 1.0.3beta

### Functionalities
- apiserver/downloader/consolidater: add --gdalNumThreads to change the -wo options of gdal.warp. 1 by default, -1 means ALL_CPUS. gdalNumThreads+workers should be lower than the number of CPUS.
- downloader: add --chunkSize (1Mbytes by default)
- gdalwarp uses wm=500 instead of 2047 and -multi option
- Min/Max to GetXYZTile to scale tile values between min and max.
- Add index on pg.records.datetime (execute interface/database/pg/update_X.X.X.sql)
- GCS: automatically retry or mark as temporary some errors 


### API
- DeleteRecords: add NoFail to delete all the pending records and let the others
- GetCube: add CompressionLevel=-3 to disable the compression
- GetCube: add Predownload option to download file before warping to save time. It is efficient when gdal needs the whole image to compute the Cube requested, but its not when a small part of the image is required. Be careful when the data has been consolidated.
- Consolidation: add collapse_on_record_id: to consolidate by collapsing all datasets on the given record (data is copied)
- Get information on containers from their uris
- FindJobs: add page/limit
- GetXYZTile support filters: records.tags, records.from_date and records.to_date (e.g. ?filters.from_time=YYYY-MM-DD&filters.to_time=YYYY-MM-DD&filters.tags[key1]=value1&filter.tags[key2]=value2...)
- Records: ? and * are not supported anymore for the record tags
- Support Int8 datatype
- DeleteDatasets is moved from Admin to Client (retrocompatibility is ensured for previous clients)

### Bug fixes
- CleanJobs: remove DONEBUTUNTIDY
- Remove redondant logs
- maxConnectionAge: default = 15min
- storage: operations retry when context is cancelled: Add utils.Retriable to test weither an error is retriable.
- Panic during dataset deletion when status is DeletionNotReady
- Consolidation used GTIFF_SUBDIR when there was no subdir
- Consolidation failed because of invalid geometry in ComputeValidShapeFromCell
- Update postGis to 3.1 to fix a bug with geography intersection (GetCube does not return all datasets)
- Docker Consolidater use uuidgen instead of ossp-uuid
- slow FindRecords
- float32 is compressed with ZSTD instead of LERC_ZSTD
- Container deletion ignores FileNotFound error
- Handling of extents that crosses dateline
- Handling of extents wider than 180° of longitude
- reindex dataset crossing antemeridian
- Not empty image are returned as empty
- Consolidation: BuildOverviews fails if GDAL raises a warning
- Consolidation: NoData=Nan does not work as expected
- generate with enumer
- AdminUpdateDataset with RecordIds
- Better handling of consolidation cancellation
- CleanJobs does not return all errors
- If nodata != none and lossy compression: use alpha band

### Others
- Update golang-mod
- Use google-cloud-go instead of go-genproto package
- Memory optimisation
- Refacto MergeDataset, using vrt.
- Dockerfile uses alpine3.21, golang:alpine3.21


## 1.0.2

### Functionalities

### Bug fixes
- countValidPix with gdal >= 3.6.0
- Deprecated api cloud.google.com/go/secretmanager/apiv1beta1 => cloud.google.com/go/secretmanager/apiv1


## 1.0.1

### Functionalities
- Consolidater: add option `--local-download` (default=`true`) to download datasets locally before starting the consolidation. Usually, it's faster to download first, but in some case, it's not (or
consume a lot of local storage)
- API: add GetRecords(List IDs)
- API: GetCube: add ResamplingAlg (override variable.ResamplingAlg)

### Bug fixes
- Cancel consolidation tasks took to much time (due to job being saved at every task)
- Update mod airbusgeo/cogger to fix a crash with overviews
- If a deletion task failed, the job must be in "DONEBUTUNTIDY" state
- Dockerfile uses alpine3.17, golang:alpine3.17
- GRPC message errors are limited to 3Kb
- Dataset bands were not taken into account during warping
