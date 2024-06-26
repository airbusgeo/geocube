# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [pb/geocube.proto](#pb_geocube-proto)
    - [Geocube](#geocube-Geocube)
  
- [pb/geocubeDownloader.proto](#pb_geocubeDownloader-proto)
    - [GeocubeDownloader](#geocube-GeocubeDownloader)
  
- [pb/admin.proto](#pb_admin-proto)
    - [DeleteDatasetsRequest](#geocube-DeleteDatasetsRequest)
    - [DeleteDatasetsResponse](#geocube-DeleteDatasetsResponse)
    - [TidyDBRequest](#geocube-TidyDBRequest)
    - [TidyDBResponse](#geocube-TidyDBResponse)
    - [UpdateDatasetsRequest](#geocube-UpdateDatasetsRequest)
    - [UpdateDatasetsResponse](#geocube-UpdateDatasetsResponse)
    - [UpdateDatasetsResponse.ResultsEntry](#geocube-UpdateDatasetsResponse-ResultsEntry)
  
    - [Admin](#geocube-Admin)
  
- [pb/records.proto](#pb_records-proto)
    - [AOI](#geocube-AOI)
    - [AddRecordsTagsRequest](#geocube-AddRecordsTagsRequest)
    - [AddRecordsTagsRequest.TagsEntry](#geocube-AddRecordsTagsRequest-TagsEntry)
    - [AddRecordsTagsResponse](#geocube-AddRecordsTagsResponse)
    - [Coord](#geocube-Coord)
    - [CreateAOIRequest](#geocube-CreateAOIRequest)
    - [CreateAOIResponse](#geocube-CreateAOIResponse)
    - [CreateRecordsRequest](#geocube-CreateRecordsRequest)
    - [CreateRecordsResponse](#geocube-CreateRecordsResponse)
    - [DeleteRecordsRequest](#geocube-DeleteRecordsRequest)
    - [DeleteRecordsResponse](#geocube-DeleteRecordsResponse)
    - [GetAOIRequest](#geocube-GetAOIRequest)
    - [GetAOIResponse](#geocube-GetAOIResponse)
    - [GetRecordsRequest](#geocube-GetRecordsRequest)
    - [GetRecordsResponseItem](#geocube-GetRecordsResponseItem)
    - [GroupedRecordIds](#geocube-GroupedRecordIds)
    - [GroupedRecordIdsList](#geocube-GroupedRecordIdsList)
    - [GroupedRecords](#geocube-GroupedRecords)
    - [LinearRing](#geocube-LinearRing)
    - [ListRecordsRequest](#geocube-ListRecordsRequest)
    - [ListRecordsRequest.TagsEntry](#geocube-ListRecordsRequest-TagsEntry)
    - [ListRecordsResponseItem](#geocube-ListRecordsResponseItem)
    - [NewRecord](#geocube-NewRecord)
    - [NewRecord.TagsEntry](#geocube-NewRecord-TagsEntry)
    - [Polygon](#geocube-Polygon)
    - [Record](#geocube-Record)
    - [Record.TagsEntry](#geocube-Record-TagsEntry)
    - [RecordFilters](#geocube-RecordFilters)
    - [RecordFilters.TagsEntry](#geocube-RecordFilters-TagsEntry)
    - [RecordFiltersWithAOI](#geocube-RecordFiltersWithAOI)
    - [RecordIdList](#geocube-RecordIdList)
    - [RemoveRecordsTagsRequest](#geocube-RemoveRecordsTagsRequest)
    - [RemoveRecordsTagsResponse](#geocube-RemoveRecordsTagsResponse)
  
- [pb/variables.proto](#pb_variables-proto)
    - [CreatePaletteRequest](#geocube-CreatePaletteRequest)
    - [CreatePaletteResponse](#geocube-CreatePaletteResponse)
    - [CreateVariableRequest](#geocube-CreateVariableRequest)
    - [CreateVariableResponse](#geocube-CreateVariableResponse)
    - [DeleteInstanceRequest](#geocube-DeleteInstanceRequest)
    - [DeleteInstanceResponse](#geocube-DeleteInstanceResponse)
    - [DeleteVariableRequest](#geocube-DeleteVariableRequest)
    - [DeleteVariableResponse](#geocube-DeleteVariableResponse)
    - [GetVariableRequest](#geocube-GetVariableRequest)
    - [GetVariableResponse](#geocube-GetVariableResponse)
    - [Instance](#geocube-Instance)
    - [Instance.MetadataEntry](#geocube-Instance-MetadataEntry)
    - [InstantiateVariableRequest](#geocube-InstantiateVariableRequest)
    - [InstantiateVariableRequest.InstanceMetadataEntry](#geocube-InstantiateVariableRequest-InstanceMetadataEntry)
    - [InstantiateVariableResponse](#geocube-InstantiateVariableResponse)
    - [ListVariablesRequest](#geocube-ListVariablesRequest)
    - [ListVariablesResponseItem](#geocube-ListVariablesResponseItem)
    - [Palette](#geocube-Palette)
    - [UpdateInstanceRequest](#geocube-UpdateInstanceRequest)
    - [UpdateInstanceRequest.AddMetadataEntry](#geocube-UpdateInstanceRequest-AddMetadataEntry)
    - [UpdateInstanceResponse](#geocube-UpdateInstanceResponse)
    - [UpdateVariableRequest](#geocube-UpdateVariableRequest)
    - [UpdateVariableResponse](#geocube-UpdateVariableResponse)
    - [Variable](#geocube-Variable)
    - [colorPoint](#geocube-colorPoint)
  
    - [Resampling](#geocube-Resampling)
  
- [pb/dataformat.proto](#pb_dataformat-proto)
    - [DataFormat](#geocube-DataFormat)
  
    - [DataFormat.Dtype](#geocube-DataFormat-Dtype)
  
- [pb/catalog.proto](#pb_catalog-proto)
    - [GetCubeMetadataRequest](#geocube-GetCubeMetadataRequest)
    - [GetCubeMetadataResponse](#geocube-GetCubeMetadataResponse)
    - [GetCubeRequest](#geocube-GetCubeRequest)
    - [GetCubeResponse](#geocube-GetCubeResponse)
    - [GetCubeResponseHeader](#geocube-GetCubeResponseHeader)
    - [GetTileRequest](#geocube-GetTileRequest)
    - [GetTileResponse](#geocube-GetTileResponse)
    - [ImageChunk](#geocube-ImageChunk)
    - [ImageFile](#geocube-ImageFile)
    - [ImageHeader](#geocube-ImageHeader)
    - [Shape](#geocube-Shape)
  
    - [ByteOrder](#geocube-ByteOrder)
    - [FileFormat](#geocube-FileFormat)
  
- [pb/layouts.proto](#pb_layouts-proto)
    - [Cell](#geocube-Cell)
    - [CreateGridRequest](#geocube-CreateGridRequest)
    - [CreateGridResponse](#geocube-CreateGridResponse)
    - [CreateLayoutRequest](#geocube-CreateLayoutRequest)
    - [CreateLayoutResponse](#geocube-CreateLayoutResponse)
    - [DeleteGridRequest](#geocube-DeleteGridRequest)
    - [DeleteGridResponse](#geocube-DeleteGridResponse)
    - [DeleteLayoutRequest](#geocube-DeleteLayoutRequest)
    - [DeleteLayoutResponse](#geocube-DeleteLayoutResponse)
    - [FindContainerLayoutsRequest](#geocube-FindContainerLayoutsRequest)
    - [FindContainerLayoutsResponse](#geocube-FindContainerLayoutsResponse)
    - [GeoTransform](#geocube-GeoTransform)
    - [Grid](#geocube-Grid)
    - [Layout](#geocube-Layout)
    - [Layout.GridParametersEntry](#geocube-Layout-GridParametersEntry)
    - [ListGridsRequest](#geocube-ListGridsRequest)
    - [ListGridsResponse](#geocube-ListGridsResponse)
    - [ListLayoutsRequest](#geocube-ListLayoutsRequest)
    - [ListLayoutsResponse](#geocube-ListLayoutsResponse)
    - [Size](#geocube-Size)
    - [Tile](#geocube-Tile)
    - [TileAOIRequest](#geocube-TileAOIRequest)
    - [TileAOIResponse](#geocube-TileAOIResponse)
  
- [pb/operations.proto](#pb_operations-proto)
    - [CancelJobRequest](#geocube-CancelJobRequest)
    - [CancelJobResponse](#geocube-CancelJobResponse)
    - [CleanJobsRequest](#geocube-CleanJobsRequest)
    - [CleanJobsResponse](#geocube-CleanJobsResponse)
    - [ConfigConsolidationRequest](#geocube-ConfigConsolidationRequest)
    - [ConfigConsolidationResponse](#geocube-ConfigConsolidationResponse)
    - [ConsolidateRequest](#geocube-ConsolidateRequest)
    - [ConsolidateResponse](#geocube-ConsolidateResponse)
    - [ConsolidationParams](#geocube-ConsolidationParams)
    - [ConsolidationParams.CreationParamsEntry](#geocube-ConsolidationParams-CreationParamsEntry)
    - [Container](#geocube-Container)
    - [ContinueJobRequest](#geocube-ContinueJobRequest)
    - [ContinueJobResponse](#geocube-ContinueJobResponse)
    - [Dataset](#geocube-Dataset)
    - [GetConsolidationParamsRequest](#geocube-GetConsolidationParamsRequest)
    - [GetConsolidationParamsResponse](#geocube-GetConsolidationParamsResponse)
    - [GetContainersRequest](#geocube-GetContainersRequest)
    - [GetContainersResponse](#geocube-GetContainersResponse)
    - [GetJobRequest](#geocube-GetJobRequest)
    - [GetJobResponse](#geocube-GetJobResponse)
    - [IndexDatasetsRequest](#geocube-IndexDatasetsRequest)
    - [IndexDatasetsResponse](#geocube-IndexDatasetsResponse)
    - [Job](#geocube-Job)
    - [ListJobsRequest](#geocube-ListJobsRequest)
    - [ListJobsResponse](#geocube-ListJobsResponse)
    - [RetryJobRequest](#geocube-RetryJobRequest)
    - [RetryJobResponse](#geocube-RetryJobResponse)
  
    - [ConsolidationParams.Compression](#geocube-ConsolidationParams-Compression)
    - [ExecutionLevel](#geocube-ExecutionLevel)
    - [StorageClass](#geocube-StorageClass)
  
- [pb/datasetMeta.proto](#pb_datasetMeta-proto)
    - [DatasetMeta](#geocube-DatasetMeta)
    - [InternalMeta](#geocube-InternalMeta)
  
- [pb/version.proto](#pb_version-proto)
    - [GetVersionRequest](#geocube-GetVersionRequest)
    - [GetVersionResponse](#geocube-GetVersionResponse)
  
- [Scalar Value Types](#scalar-value-types)



<a name="pb_geocube-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/geocube.proto


 

 

 


<a name="geocube-Geocube"></a>

### Geocube
API
Documentation may be detailed in Request/Response sections.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateRecords | [CreateRecordsRequest](#geocube-CreateRecordsRequest) | [CreateRecordsResponse](#geocube-CreateRecordsResponse) | Create one or a list of records |
| GetRecords | [GetRecordsRequest](#geocube-GetRecordsRequest) | [GetRecordsResponseItem](#geocube-GetRecordsResponseItem) stream | Get records from their ID |
| ListRecords | [ListRecordsRequest](#geocube-ListRecordsRequest) | [ListRecordsResponseItem](#geocube-ListRecordsResponseItem) stream | List records given criterias |
| AddRecordsTags | [AddRecordsTagsRequest](#geocube-AddRecordsTagsRequest) | [AddRecordsTagsResponse](#geocube-AddRecordsTagsResponse) | Update records, adding or updating tags |
| RemoveRecordsTags | [RemoveRecordsTagsRequest](#geocube-RemoveRecordsTagsRequest) | [RemoveRecordsTagsResponse](#geocube-RemoveRecordsTagsResponse) | Update records, removing tags |
| DeleteRecords | [DeleteRecordsRequest](#geocube-DeleteRecordsRequest) | [DeleteRecordsResponse](#geocube-DeleteRecordsResponse) | Delete records |
| CreateAOI | [CreateAOIRequest](#geocube-CreateAOIRequest) | [CreateAOIResponse](#geocube-CreateAOIResponse) | Create an AOI if not exists or returns the aoi id of the aoi. |
| GetAOI | [GetAOIRequest](#geocube-GetAOIRequest) | [GetAOIResponse](#geocube-GetAOIResponse) | Get an AOI from its ID |
| CreateVariable | [CreateVariableRequest](#geocube-CreateVariableRequest) | [CreateVariableResponse](#geocube-CreateVariableResponse) | Create a variable |
| GetVariable | [GetVariableRequest](#geocube-GetVariableRequest) | [GetVariableResponse](#geocube-GetVariableResponse) | Get a variable given its id, name or one of its instance id |
| UpdateVariable | [UpdateVariableRequest](#geocube-UpdateVariableRequest) | [UpdateVariableResponse](#geocube-UpdateVariableResponse) | Update some fields of a variable |
| DeleteVariable | [DeleteVariableRequest](#geocube-DeleteVariableRequest) | [DeleteVariableResponse](#geocube-DeleteVariableResponse) | Delete a variable iif no dataset has a reference on |
| ListVariables | [ListVariablesRequest](#geocube-ListVariablesRequest) | [ListVariablesResponseItem](#geocube-ListVariablesResponseItem) stream | List variables given a name pattern |
| InstantiateVariable | [InstantiateVariableRequest](#geocube-InstantiateVariableRequest) | [InstantiateVariableResponse](#geocube-InstantiateVariableResponse) | Instantiate a variable |
| UpdateInstance | [UpdateInstanceRequest](#geocube-UpdateInstanceRequest) | [UpdateInstanceResponse](#geocube-UpdateInstanceResponse) | Update metadata of an instance |
| DeleteInstance | [DeleteInstanceRequest](#geocube-DeleteInstanceRequest) | [DeleteInstanceResponse](#geocube-DeleteInstanceResponse) | Delete an instance iif no dataset has a reference on |
| CreatePalette | [CreatePaletteRequest](#geocube-CreatePaletteRequest) | [CreatePaletteResponse](#geocube-CreatePaletteResponse) | Create or update a palette that can be used to create a display of a dataset |
| GetContainers | [GetContainersRequest](#geocube-GetContainersRequest) | [GetContainersResponse](#geocube-GetContainersResponse) | GetInfo on containers |
| IndexDatasets | [IndexDatasetsRequest](#geocube-IndexDatasetsRequest) | [IndexDatasetsResponse](#geocube-IndexDatasetsResponse) | Index new datasets in the Geocube |
| ConfigConsolidation | [ConfigConsolidationRequest](#geocube-ConfigConsolidationRequest) | [ConfigConsolidationResponse](#geocube-ConfigConsolidationResponse) | Configurate a consolidation process |
| GetConsolidationParams | [GetConsolidationParamsRequest](#geocube-GetConsolidationParamsRequest) | [GetConsolidationParamsResponse](#geocube-GetConsolidationParamsResponse) | Get the configuration of a consolidation |
| Consolidate | [ConsolidateRequest](#geocube-ConsolidateRequest) | [ConsolidateResponse](#geocube-ConsolidateResponse) | Start a consolidation job |
| ListJobs | [ListJobsRequest](#geocube-ListJobsRequest) | [ListJobsResponse](#geocube-ListJobsResponse) | List the jobs given a name pattern |
| GetJob | [GetJobRequest](#geocube-GetJobRequest) | [GetJobResponse](#geocube-GetJobResponse) | Get a job given its name |
| CleanJobs | [CleanJobsRequest](#geocube-CleanJobsRequest) | [CleanJobsResponse](#geocube-CleanJobsResponse) | Delete jobs given their status |
| RetryJob | [RetryJobRequest](#geocube-RetryJobRequest) | [RetryJobResponse](#geocube-RetryJobResponse) | Retry a job |
| CancelJob | [CancelJobRequest](#geocube-CancelJobRequest) | [CancelJobResponse](#geocube-CancelJobResponse) | Cancel a job |
| ContinueJob | [ContinueJobRequest](#geocube-ContinueJobRequest) | [ContinueJobResponse](#geocube-ContinueJobResponse) | Continue a job that is in waiting state |
| GetCube | [GetCubeRequest](#geocube-GetCubeRequest) | [GetCubeResponse](#geocube-GetCubeResponse) stream | Get a cube of data given a CubeParams |
| GetXYZTile | [GetTileRequest](#geocube-GetTileRequest) | [GetTileResponse](#geocube-GetTileResponse) | Get a XYZTile (can be used with a TileServer, provided a GRPCGateway is up) |
| CreateLayout | [CreateLayoutRequest](#geocube-CreateLayoutRequest) | [CreateLayoutResponse](#geocube-CreateLayoutResponse) | Create a layout to be used for tiling or consolidation |
| DeleteLayout | [DeleteLayoutRequest](#geocube-DeleteLayoutRequest) | [DeleteLayoutResponse](#geocube-DeleteLayoutResponse) | Delete a layout given its name |
| ListLayouts | [ListLayoutsRequest](#geocube-ListLayoutsRequest) | [ListLayoutsResponse](#geocube-ListLayoutsResponse) | List layouts given a name pattern |
| FindContainerLayouts | [FindContainerLayoutsRequest](#geocube-FindContainerLayoutsRequest) | [FindContainerLayoutsResponse](#geocube-FindContainerLayoutsResponse) stream | Find all the layouts known for a set of containers |
| TileAOI | [TileAOIRequest](#geocube-TileAOIRequest) | [TileAOIResponse](#geocube-TileAOIResponse) stream | Tile an AOI given a layout |
| CreateGrid | [CreateGridRequest](#geocube-CreateGridRequest) stream | [CreateGridResponse](#geocube-CreateGridResponse) | Create a grid that can be used to tile an AOI |
| DeleteGrid | [DeleteGridRequest](#geocube-DeleteGridRequest) | [DeleteGridResponse](#geocube-DeleteGridResponse) | Delete a grid |
| ListGrids | [ListGridsRequest](#geocube-ListGridsRequest) | [ListGridsResponse](#geocube-ListGridsResponse) | List grids given a name pattern |
| Version | [GetVersionRequest](#geocube-GetVersionRequest) | [GetVersionResponse](#geocube-GetVersionResponse) | Version of the GeocubeServer |

 



<a name="pb_geocubeDownloader-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/geocubeDownloader.proto


 

 

 


<a name="geocube-GeocubeDownloader"></a>

### GeocubeDownloader
API GeocubeDownloader to download a cube from metadata

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| DownloadCube | [GetCubeMetadataRequest](#geocube-GetCubeMetadataRequest) | [GetCubeMetadataResponse](#geocube-GetCubeMetadataResponse) stream | Request cube using metadatas returned by a call to Geocube.GetCube() |
| Version | [GetVersionRequest](#geocube-GetVersionRequest) | [GetVersionResponse](#geocube-GetVersionResponse) | Version of the GeocubeDownloader |

 



<a name="pb_admin-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/admin.proto



<a name="geocube-DeleteDatasetsRequest"></a>

### DeleteDatasetsRequest
Remove the datasets referenced by instances and records without any control
The containers (if empty) are not deleted


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| record_ids | [string](#string) | repeated | bool simulate = 1; // DEPRECATED If true, a simulation is done, nothing is actually deleted. Use StepByStep=2 instead

Instance id that references the datasets to be deleted |
| instance_ids | [string](#string) | repeated | Record ids that reference the datasets to be deleted |
| dataset_patterns | [string](#string) | repeated | Dataset file patterns (support * and ? for all or any characters and trailing (?i) for case-insensitiveness) (or empty to ignore) |
| execution_level | [ExecutionLevel](#geocube-ExecutionLevel) |  | Execution level (see enum) |
| job_name | [string](#string) |  | Name of the job (if empty, a name will be generated) |






<a name="geocube-DeleteDatasetsResponse"></a>

### DeleteDatasetsResponse
Return the number of deleted datasets


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| job | [Job](#geocube-Job) |  | repeated string results = 1; // DEPRECATED: use the log of the job |






<a name="geocube-TidyDBRequest"></a>

### TidyDBRequest
Request to remove from the database all the pending entities (entities that are not linked to any dataset)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Simulate | [bool](#bool) |  | If true, a simulation is done, nothing is actually deleted |
| PendingAOIs | [bool](#bool) |  | Remove AOIs that are not linked to any Records |
| PendingRecords | [bool](#bool) |  | Remove Records that do not reference any Datasets |
| PendingVariables | [bool](#bool) |  | Remove Variables that have not any instances |
| PendingInstances | [bool](#bool) |  | Remove Instances that do not reference any Datasets |
| PendingContainers | [bool](#bool) |  | Remove Containers that do not contain any Datasets |
| PendingParams | [bool](#bool) |  | Remove ConsolidationParams that are not linked to any Variable or Job |






<a name="geocube-TidyDBResponse"></a>

### TidyDBResponse
Return the number of entities that were deleted (or should have been deleted if Simulate=True)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| NbAOIs | [int64](#int64) |  |  |
| NbRecords | [int64](#int64) |  |  |
| NbVariables | [int64](#int64) |  |  |
| NbInstances | [int64](#int64) |  |  |
| NbContainers | [int64](#int64) |  |  |
| NbParams | [int64](#int64) |  |  |






<a name="geocube-UpdateDatasetsRequest"></a>

### UpdateDatasetsRequest
Update fields of datasets that can be tricky


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| simulate | [bool](#bool) |  | If true, a simulation is done, nothing is actually updated |
| instance_id | [string](#string) |  | Instance id that references the datasets to be updated |
| record_ids | [string](#string) | repeated | Record ids that reference the datasets to be updated |
| dformat | [DataFormat](#geocube-DataFormat) |  | Internal data format (DType can be Undefined) |
| real_min_value | [double](#double) |  | Real min value (dformat.min_value maps to real_min_value) |
| real_max_value | [double](#double) |  | Real max value (dformat.max_value maps to real_max_value) |
| exponent | [double](#double) |  | 1: linear scaling (RealMax - RealMin) * pow( (Value - Min) / (Max - Min), Exponent) &#43; RealMin |






<a name="geocube-UpdateDatasetsResponse"></a>

### UpdateDatasetsResponse
Return the number of modifications per kind of modification


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| results | [UpdateDatasetsResponse.ResultsEntry](#geocube-UpdateDatasetsResponse-ResultsEntry) | repeated |  |






<a name="geocube-UpdateDatasetsResponse-ResultsEntry"></a>

### UpdateDatasetsResponse.ResultsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [int64](#int64) |  |  |





 

 

 


<a name="geocube-Admin"></a>

### Admin
Service providing some functions to update or clean the database
Must be used cautiously because there is no control neither possible rollback

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| TidyDB | [TidyDBRequest](#geocube-TidyDBRequest) | [TidyDBResponse](#geocube-TidyDBResponse) |  |
| UpdateDatasets | [UpdateDatasetsRequest](#geocube-UpdateDatasetsRequest) | [UpdateDatasetsResponse](#geocube-UpdateDatasetsResponse) |  |
| DeleteDatasets | [DeleteDatasetsRequest](#geocube-DeleteDatasetsRequest) | [DeleteDatasetsResponse](#geocube-DeleteDatasetsResponse) |  |

 



<a name="pb_records-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/records.proto



<a name="geocube-AOI"></a>

### AOI
Geographic AOI


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| polygons | [Polygon](#geocube-Polygon) | repeated |  |






<a name="geocube-AddRecordsTagsRequest"></a>

### AddRecordsTagsRequest
Add the given tags to a set of records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |
| tags | [AddRecordsTagsRequest.TagsEntry](#geocube-AddRecordsTagsRequest-TagsEntry) | repeated |  |






<a name="geocube-AddRecordsTagsRequest-TagsEntry"></a>

### AddRecordsTagsRequest.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-AddRecordsTagsResponse"></a>

### AddRecordsTagsResponse
Returns the number of records impacted by the addition


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nb | [int64](#int64) |  |  |






<a name="geocube-Coord"></a>

### Coord
Geographic coordinates (4326)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lon | [float](#float) |  |  |
| lat | [float](#float) |  |  |






<a name="geocube-CreateAOIRequest"></a>

### CreateAOIRequest
Create a new AOI


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| aoi | [AOI](#geocube-AOI) |  |  |






<a name="geocube-CreateAOIResponse"></a>

### CreateAOIResponse
Returns the ID of the AOI


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="geocube-CreateRecordsRequest"></a>

### CreateRecordsRequest
Create new records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| records | [NewRecord](#geocube-NewRecord) | repeated |  |






<a name="geocube-CreateRecordsResponse"></a>

### CreateRecordsResponse
Returns the ID of the created records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |






<a name="geocube-DeleteRecordsRequest"></a>

### DeleteRecordsRequest
Delete records by ID


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |
| no_fail | [bool](#bool) |  | If true, do not fail if some records still have datasets that refer to them and delete the others. |






<a name="geocube-DeleteRecordsResponse"></a>

### DeleteRecordsResponse
Return the number of deleted records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nb | [int64](#int64) |  |  |






<a name="geocube-GetAOIRequest"></a>

### GetAOIRequest
Request the AOI given its ID


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="geocube-GetAOIResponse"></a>

### GetAOIResponse
Returns a geometric AOI


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| aoi | [AOI](#geocube-AOI) |  |  |






<a name="geocube-GetRecordsRequest"></a>

### GetRecordsRequest
Get record from its id


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |






<a name="geocube-GetRecordsResponseItem"></a>

### GetRecordsResponseItem
Return a record


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| record | [Record](#geocube-Record) |  |  |






<a name="geocube-GroupedRecordIds"></a>

### GroupedRecordIds
Record ids that are considered as a unique, merged record (e.g. all records of a given date, whatever the time of the day)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |






<a name="geocube-GroupedRecordIdsList"></a>

### GroupedRecordIdsList
List of groupedRecordIds


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| records | [GroupedRecordIds](#geocube-GroupedRecordIds) | repeated |  |






<a name="geocube-GroupedRecords"></a>

### GroupedRecords
Records that are considered as a unique, merged record (e.g. all records of a given date, whatever the time of the day)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| records | [Record](#geocube-Record) | repeated |  |






<a name="geocube-LinearRing"></a>

### LinearRing
Geographic linear ring


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| points | [Coord](#geocube-Coord) | repeated |  |






<a name="geocube-ListRecordsRequest"></a>

### ListRecordsRequest
Request to find the list of records corresponding to multiple filters (inclusive)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Name pattern (support * and ? for all or any characters and trailing (?i) for case-insensitiveness) |
| tags | [ListRecordsRequest.TagsEntry](#geocube-ListRecordsRequest-TagsEntry) | repeated | cf RecordFilters |
| from_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | cf RecordFilters |
| to_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | cf RecordFilters |
| aoi | [AOI](#geocube-AOI) |  | cf RecordFiltersWithAOI |
| limit | [int32](#int32) |  |  |
| page | [int32](#int32) |  |  |
| with_aoi | [bool](#bool) |  | Also returns the AOI (may be big) |






<a name="geocube-ListRecordsRequest-TagsEntry"></a>

### ListRecordsRequest.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-ListRecordsResponseItem"></a>

### ListRecordsResponseItem



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| record | [Record](#geocube-Record) |  |  |






<a name="geocube-NewRecord"></a>

### NewRecord
Structure to create a new record


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| tags | [NewRecord.TagsEntry](#geocube-NewRecord-TagsEntry) | repeated |  |
| aoi_id | [string](#string) |  |  |






<a name="geocube-NewRecord-TagsEntry"></a>

### NewRecord.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-Polygon"></a>

### Polygon
Geographic polygon


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| linearrings | [LinearRing](#geocube-LinearRing) | repeated |  |






<a name="geocube-Record"></a>

### Record
Record


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  |  |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| tags | [Record.TagsEntry](#geocube-Record-TagsEntry) | repeated |  |
| aoi_id | [string](#string) |  |  |
| aoi | [AOI](#geocube-AOI) |  | optional |






<a name="geocube-Record-TagsEntry"></a>

### Record.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-RecordFilters"></a>

### RecordFilters
RecordFilters defines some filters to identify records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tags | [RecordFilters.TagsEntry](#geocube-RecordFilters-TagsEntry) | repeated | Tags of the records |
| from_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Minimum date of the records |
| to_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Maximum date of the records |






<a name="geocube-RecordFilters-TagsEntry"></a>

### RecordFilters.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-RecordFiltersWithAOI"></a>

### RecordFiltersWithAOI
RecordFiltersWithAOI defines some filters to identify records, including an AOI in geometric coordinates


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filters | [RecordFilters](#geocube-RecordFilters) |  |  |
| aoi | [AOI](#geocube-AOI) |  | Geometric coordinates of an AOI that intersects the AOI of the records |






<a name="geocube-RecordIdList"></a>

### RecordIdList
List of record ids that are considered separately


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |






<a name="geocube-RemoveRecordsTagsRequest"></a>

### RemoveRecordsTagsRequest
Remove the given tags for a set of records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |
| tagsKey | [string](#string) | repeated |  |






<a name="geocube-RemoveRecordsTagsResponse"></a>

### RemoveRecordsTagsResponse
Returns the number of records impacted by the removal


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nb | [int64](#int64) |  |  |





 

 

 

 



<a name="pb_variables-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/variables.proto



<a name="geocube-CreatePaletteRequest"></a>

### CreatePaletteRequest
Create a new palette or update it if already exists (provided replace=True)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| palette | [Palette](#geocube-Palette) |  | Palette to be created |
| replace | [bool](#bool) |  | Replace the current existing palette if exists |






<a name="geocube-CreatePaletteResponse"></a>

### CreatePaletteResponse
Return nothing.






<a name="geocube-CreateVariableRequest"></a>

### CreateVariableRequest
Define a new variable.
Return an error if the name already exists.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable | [Variable](#geocube-Variable) |  |  |






<a name="geocube-CreateVariableResponse"></a>

### CreateVariableResponse
Return the id of the new variable.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="geocube-DeleteInstanceRequest"></a>

### DeleteInstanceRequest
Delete an instance
Return an error if the instance is linked to datasets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | UUID-4 of the instance to delete |






<a name="geocube-DeleteInstanceResponse"></a>

### DeleteInstanceResponse
Return nothing






<a name="geocube-DeleteVariableRequest"></a>

### DeleteVariableRequest
Delete a variable
Return an error if the variable has still instances


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | UUID-4 of the variable to delete |






<a name="geocube-DeleteVariableResponse"></a>

### DeleteVariableResponse
Return nothing






<a name="geocube-GetVariableRequest"></a>

### GetVariableRequest
Read a variable given either its id, its name or the id of one of its instance


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | UUID-4 of the variable |
| name | [string](#string) |  | Name of the variable |
| instance_id | [string](#string) |  | UUID-4 of an instance |






<a name="geocube-GetVariableResponse"></a>

### GetVariableResponse
Return the variable and its instances


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable | [Variable](#geocube-Variable) |  |  |






<a name="geocube-Instance"></a>

### Instance



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Null at creation |
| name | [string](#string) |  |  |
| metadata | [Instance.MetadataEntry](#geocube-Instance-MetadataEntry) | repeated |  |






<a name="geocube-Instance-MetadataEntry"></a>

### Instance.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-InstantiateVariableRequest"></a>

### InstantiateVariableRequest
Instantiate a variable.
Return an error if the instance_name already exists for this variable.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable_id | [string](#string) |  |  |
| instance_name | [string](#string) |  |  |
| instance_metadata | [InstantiateVariableRequest.InstanceMetadataEntry](#geocube-InstantiateVariableRequest-InstanceMetadataEntry) | repeated |  |






<a name="geocube-InstantiateVariableRequest-InstanceMetadataEntry"></a>

### InstantiateVariableRequest.InstanceMetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-InstantiateVariableResponse"></a>

### InstantiateVariableResponse
Return the new instance (its id, name and metadata)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instance | [Instance](#geocube-Instance) |  |  |






<a name="geocube-ListVariablesRequest"></a>

### ListVariablesRequest
List variables given a name pattern


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Pattern of the name of the variable (support * and ? for all or any characters, (?i) suffix for case-insensitiveness) |
| limit | [int32](#int32) |  | Limit the number of variables returned |
| page | [int32](#int32) |  | Navigate through results (start at 0) |






<a name="geocube-ListVariablesResponseItem"></a>

### ListVariablesResponseItem
Return a stream of variables


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable | [Variable](#geocube-Variable) |  |  |






<a name="geocube-Palette"></a>

### Palette
Define a palette with a name and a set of colorPoint.
Maps all values in [0,1] to an RGBA value, using piecewise curve defined by colorPoints.
All intermediate values are linearly interpolated.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Name of the palette (Alpha-numerics characters, dots, dashes and underscores are supported) |
| colors | [colorPoint](#geocube-colorPoint) | repeated | Set of colorPoints. At least two points must be defined, corresponding to value=0 and value=1. |






<a name="geocube-UpdateInstanceRequest"></a>

### UpdateInstanceRequest
Update an instance
Return an error if the name is to be updated but the new name already exists.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | UUID-4 of the instance to update |
| name | [google.protobuf.StringValue](#google-protobuf-StringValue) |  | [Optional] New name of the variable. Empty to ignore |
| add_metadata | [UpdateInstanceRequest.AddMetadataEntry](#geocube-UpdateInstanceRequest-AddMetadataEntry) | repeated | Pairs of metadata (key, values) to be inserted or updated |
| del_metadata_keys | [string](#string) | repeated | Metadata keys to be deleted |






<a name="geocube-UpdateInstanceRequest-AddMetadataEntry"></a>

### UpdateInstanceRequest.AddMetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-UpdateInstanceResponse"></a>

### UpdateInstanceResponse
Return nothing






<a name="geocube-UpdateVariableRequest"></a>

### UpdateVariableRequest
Update the non-critical fields of a variable
Return an error if the name is to be updated but the new name already exists.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | UUID-4 of the variable to update |
| name | [google.protobuf.StringValue](#google-protobuf-StringValue) |  | [Optional] New name of the variable. Empty to ignore |
| unit | [google.protobuf.StringValue](#google-protobuf-StringValue) |  | [Optional] New unit of the variable. Empty to ignore |
| description | [google.protobuf.StringValue](#google-protobuf-StringValue) |  | [Optional] New description of the variable. Empty to ignore |
| palette | [google.protobuf.StringValue](#google-protobuf-StringValue) |  | [Optional] New default palette of the variable. Empty to ignore |
| resampling_alg | [Resampling](#geocube-Resampling) |  | [Optional] New default resampling algorithm of the variable. UNDEFINED to ignore |






<a name="geocube-UpdateVariableResponse"></a>

### UpdateVariableResponse
Return nothing






<a name="geocube-Variable"></a>

### Variable
Variable


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Internal UUID-4 of the variable (ignored at creation) |
| name | [string](#string) |  | Name of the variable (Alpha-numerics characters, dashs, dots and underscores) |
| unit | [string](#string) |  | Unit of the variable (for user information only) |
| description | [string](#string) |  | Description of the variable (for user information only) |
| dformat | [DataFormat](#geocube-DataFormat) |  | Format of the data. Range.Min and Range.Max are used for data mapping from internal data format of a dataset (See IndexDatasets for more details), DType and NoData are used for the outputs of GetCube. |
| bands | [string](#string) | repeated | Name of each band. Can be empty when the variable refers to only one band, must be unique otherwise. |
| palette | [string](#string) |  | Name of the default palette for color rendering. |
| resampling_alg | [Resampling](#geocube-Resampling) |  | Default resampling algorithm in case of reprojection. |
| instances | [Instance](#geocube-Instance) | repeated | List of instances of the variable (ignored at creation) |






<a name="geocube-colorPoint"></a>

### colorPoint
Define a color mapping from a value [0-1] to a RGBA value.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| value | [float](#float) |  |  |
| r | [uint32](#uint32) |  |  |
| g | [uint32](#uint32) |  |  |
| b | [uint32](#uint32) |  |  |
| a | [uint32](#uint32) |  |  |





 


<a name="geocube-Resampling"></a>

### Resampling
Resampling algorithms (supported by GDAL)

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNDEFINED | 0 |  |
| NEAR | 1 |  |
| BILINEAR | 2 |  |
| CUBIC | 3 |  |
| CUBICSPLINE | 4 |  |
| LANCZOS | 5 |  |
| AVERAGE | 6 |  |
| MODE | 7 |  |
| MAX | 8 |  |
| MIN | 9 |  |
| MED | 10 |  |
| Q1 | 11 |  |
| Q3 | 12 |  |


 

 

 



<a name="pb_dataformat-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/dataformat.proto



<a name="geocube-DataFormat"></a>

### DataFormat
Format of the data of a dataset.
Format is defined by the type of the data, its no-data value and the range of values (its interpretation depends on the use)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dtype | [DataFormat.Dtype](#geocube-DataFormat-Dtype) |  | Type of the data |
| no_data | [double](#double) |  | No-data value (supports any float values, including NaN) |
| min_value | [double](#double) |  | Min value (usually used to map from one min value to another) |
| max_value | [double](#double) |  | Max value (usually used to map from one min value to another) |





 


<a name="geocube-DataFormat-Dtype"></a>

### DataFormat.Dtype
Type of data supported by the Geocube &amp; GDAL

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNDEFINED | 0 |  |
| UInt8 | 1 |  |
| UInt16 | 2 |  |
| UInt32 | 3 |  |
| Int8 | 4 |  |
| Int16 | 5 |  |
| Int32 | 6 |  |
| Float32 | 7 |  |
| Float64 | 8 |  |
| Complex64 | 9 | Pair of float32 |


 

 

 



<a name="pb_catalog-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/catalog.proto



<a name="geocube-GetCubeMetadataRequest"></a>

### GetCubeMetadataRequest
Request a cube from metadatas (provided by Geocube.GetCube())


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| datasets_meta | [DatasetMeta](#geocube-DatasetMeta) | repeated | List of Metadatas needed to download and generate the slices of the cube |
| grouped_records | [GroupedRecords](#geocube-GroupedRecords) | repeated | List of GroupedRecords describing the slices of the cube |
| ref_dformat | [DataFormat](#geocube-DataFormat) |  | Output dataformat |
| resampling_alg | [Resampling](#geocube-Resampling) |  | Resampling algorithm to use for reprojection |
| pix_to_crs | [GeoTransform](#geocube-GeoTransform) |  |  |
| crs | [string](#string) |  |  |
| size | [Size](#geocube-Size) |  |  |
| format | [FileFormat](#geocube-FileFormat) |  | Format of the output data |
| predownload | [bool](#bool) |  | Predownload the datasets before merging them. When the dataset is remote and all the dataset is required, it is more efficient to predownload it. |






<a name="geocube-GetCubeMetadataResponse"></a>

### GetCubeMetadataResponse
Return either information on the cube, information on an image or a chunk of an image


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| global_header | [GetCubeResponseHeader](#geocube-GetCubeResponseHeader) |  |  |
| header | [ImageHeader](#geocube-ImageHeader) |  |  |
| chunk | [ImageChunk](#geocube-ImageChunk) |  |  |






<a name="geocube-GetCubeRequest"></a>

### GetCubeRequest
Request a cube of data


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| records | [RecordIdList](#geocube-RecordIdList) |  | List of record ids requested. At least one. One image will be returned by record (if not empty) |
| filters | [RecordFilters](#geocube-RecordFilters) |  | Filters to list the records that will be used to create the cube |
| grouped_records | [GroupedRecordIdsList](#geocube-GroupedRecordIdsList) |  | List of groups of record ids requested. At least one. One image will be returned by group of records (if not empty). All the datasets of a group of records will be merged together using the latest first. |
| instances_id | [string](#string) | repeated | Instances of a variable defining the kind of images requested. At least one, and all must be instance of the same variable. Only one is actually supported |
| crs | [string](#string) |  | Coordinates Reference System of the output images (images will be reprojected on the fly if necessary) |
| pix_to_crs | [GeoTransform](#geocube-GeoTransform) |  | GeoTransform of the requested cube (images will be rescaled on the fly if necessary) |
| size | [Size](#geocube-Size) |  | Shape of the output images |
| compression_level | [int32](#int32) |  | Define a level of compression to speed up the transfer, values: -3 to 9 (-2: Huffman only, -1:default, 0-&gt;9: level of compression from the fastest to the best compression, -3: disable the compression). The data is compressed by the server and decompressed by the Client. Use -3 or -2 if the bandwidth is not limited. 0 is level 0 of DEFLATE (thus, it must be decompressed by DEFLATE even though the data is not compressed). If the client can support -3, 0 is useless. |
| headers_only | [bool](#bool) |  | Only returns headers (including all metadatas on datasets) |
| format | [FileFormat](#geocube-FileFormat) |  | Format of the output images |
| resampling_alg | [Resampling](#geocube-Resampling) |  | Resampling algorithm used for reprojecion. If undefined, the default resampling algorithm associated to the variable is used. |






<a name="geocube-GetCubeResponse"></a>

### GetCubeResponse
Return either information on the cube, information on an image or a chunk of an image


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| global_header | [GetCubeResponseHeader](#geocube-GetCubeResponseHeader) |  |  |
| header | [ImageHeader](#geocube-ImageHeader) |  |  |
| chunk | [ImageChunk](#geocube-ImageChunk) |  |  |






<a name="geocube-GetCubeResponseHeader"></a>

### GetCubeResponseHeader
Return global information on the requested cube


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int64](#int64) |  |  |
| nb_datasets | [int64](#int64) |  |  |
| ref_dformat | [DataFormat](#geocube-DataFormat) |  | Output dataformat |
| resampling_alg | [Resampling](#geocube-Resampling) |  | Resampling algorithm to use for reprojection |
| geotransform | [GeoTransform](#geocube-GeoTransform) |  | Geotransform used for mapping |
| crs | [string](#string) |  |  |






<a name="geocube-GetTileRequest"></a>

### GetTileRequest
Request a web-mercator tile, given a variable and a group of records


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instance_id | [string](#string) |  |  |
| x | [int32](#int32) |  |  |
| y | [int32](#int32) |  |  |
| z | [int32](#int32) |  |  |
| min | [float](#float) |  |  |
| max | [float](#float) |  |  |
| records | [GroupedRecordIds](#geocube-GroupedRecordIds) |  | Group of record ids. At least one. All the datasets of the group of records will be merged together using the latest first. |
| filters | [RecordFilters](#geocube-RecordFilters) |  | All the datasets whose records have RecordTags and time between from_time and to_time |






<a name="geocube-GetTileResponse"></a>

### GetTileResponse
Return a 256x256 png image


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [ImageFile](#geocube-ImageFile) |  |  |






<a name="geocube-ImageChunk"></a>

### ImageChunk
Chunk of the full image, to handle the GRPC limit of 4Mbytes/message


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| part | [int32](#int32) |  | Index of the chunk (from 1 to ImageHeader.nb_parts-1). The first part (=0) is ImageHeader.data |
| data | [bytes](#bytes) |  | Chunk of the full array of bytes |






<a name="geocube-ImageFile"></a>

### ImageFile
ByteArray of a PNG image 256x256pixels


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [bytes](#bytes) |  |  |






<a name="geocube-ImageHeader"></a>

### ImageHeader
Header of an image (slice of the cube)
It describes the image, the underlying datasets and the way to recreate it from the array of byte :
1. Append ImageHeader.data and ImageChunk.data from part=0 to part=nb_parts-1
2. If compression=True, decompress the array of bytes using deflate
3. Cast the result to the dtype using byteOrder
4. Reshape the result


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| shape | [Shape](#geocube-Shape) |  | Shape of the image (widthxheight) |
| dtype | [DataFormat.Dtype](#geocube-DataFormat-Dtype) |  | Type of the data (to interprete &#34;ImageHeader.data &#43; ImageChunk.data&#34;) |
| nb_parts | [int32](#int32) |  | Number of parts the image is splitted into |
| data | [bytes](#bytes) |  | First part of the image as an array of bytes |
| size | [int64](#int64) |  | Size of the full array of bytes |
| order | [ByteOrder](#geocube-ByteOrder) |  | ByteOrder of the datatype |
| compression | [bool](#bool) |  | Deflate compressed data format, described in RFC 1951 |
| grouped_records | [GroupedRecords](#geocube-GroupedRecords) |  | Group of records used to generate this image |
| dataset_meta | [DatasetMeta](#geocube-DatasetMeta) |  | All information on the underlying datasets that composed the image |
| error | [string](#string) |  | If not empty, an error occured and the image was not retrieved. |






<a name="geocube-Shape"></a>

### Shape
Shape of an image width x height x channels


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dim1 | [int32](#int32) |  |  |
| dim2 | [int32](#int32) |  |  |
| dim3 | [int32](#int32) |  |  |





 


<a name="geocube-ByteOrder"></a>

### ByteOrder
ByteOrder for the conversion between data type and byte.

| Name | Number | Description |
| ---- | ------ | ----------- |
| LittleEndian | 0 |  |
| BigEndian | 1 |  |



<a name="geocube-FileFormat"></a>

### FileFormat
Available file formats

| Name | Number | Description |
| ---- | ------ | ----------- |
| Raw | 0 | raw bitmap |
| GTiff | 1 |  |


 

 

 



<a name="pb_layouts-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/layouts.proto



<a name="geocube-Cell"></a>

### Cell
Define a cell of a grid


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Cell identifier |
| crs | [string](#string) |  | Coordinate reference system used in the cell |
| coordinates | [LinearRing](#geocube-LinearRing) |  | Geographic coordinates |






<a name="geocube-CreateGridRequest"></a>

### CreateGridRequest
Create a new grid.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| grid | [Grid](#geocube-Grid) |  |  |






<a name="geocube-CreateGridResponse"></a>

### CreateGridResponse







<a name="geocube-CreateLayoutRequest"></a>

### CreateLayoutRequest
Create a new layout
Return an error if the name already exists


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| layout | [Layout](#geocube-Layout) |  |  |






<a name="geocube-CreateLayoutResponse"></a>

### CreateLayoutResponse







<a name="geocube-DeleteGridRequest"></a>

### DeleteGridRequest
Delete a grid


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |






<a name="geocube-DeleteGridResponse"></a>

### DeleteGridResponse







<a name="geocube-DeleteLayoutRequest"></a>

### DeleteLayoutRequest
Delete a layout by name


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |






<a name="geocube-DeleteLayoutResponse"></a>

### DeleteLayoutResponse







<a name="geocube-FindContainerLayoutsRequest"></a>

### FindContainerLayoutsRequest
Find all the layouts used by the datasets on an AOI or a set of records
It can be used to tile the AOI with an optimal layout.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instance_id | [string](#string) |  |  |
| records | [RecordIdList](#geocube-RecordIdList) |  | List of record ids |
| filters | [RecordFiltersWithAOI](#geocube-RecordFiltersWithAOI) |  | Filters to select records |






<a name="geocube-FindContainerLayoutsResponse"></a>

### FindContainerLayoutsResponse
Stream the name of the layout and the associated containers


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| layout_name | [string](#string) |  | Name of the layout |
| container_uris | [string](#string) | repeated | List of containers having the layout |






<a name="geocube-GeoTransform"></a>

### GeoTransform
GDAL GeoTransform


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| a | [double](#double) |  | x offset |
| b | [double](#double) |  | x resolution |
| c | [double](#double) |  |  |
| d | [double](#double) |  | y offset |
| e | [double](#double) |  |  |
| f | [double](#double) |  | y resolution |






<a name="geocube-Grid"></a>

### Grid
Define a grid


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Unique name of the grid |
| description | [string](#string) |  | Description of the grid |
| cells | [Cell](#geocube-Cell) | repeated | Cells of the grid |






<a name="geocube-Layout"></a>

### Layout
Define a layout for consolidation. A layout is composed of an external and an internal layout.
External layout is a grid that is used to cover any area with tiles.
TODO Internal layout defines the internal structure of a dataset

Interlacing_pattern defines how to interlace the [R]ecords, the [B]ands, the [Z]ooms level/overview and the [T]iles (geotiff blocks).
The four levels of interlacing must be prioritized in the following way L1&gt;L2&gt;L3&gt;L4 where each L is in [R, B, Z, T]. This order should be understood as:
for each L1:
  for each L2:
    for each L3:
      for each L4:
        addBlock(L1, L2, L3, L4)
In other words, all L4 for a given (L1, L2, L3) will be contiguous in memory.
For example:
- To optimize the access to geographical information of all the bands (such as in COG) : R&gt;Z&gt;T&gt;B  =&gt; For a given record, zoom level and block, all the bands will be contiguous.
- To optimize the access to geographical information of one band at a time : B&gt;R&gt;Z&gt;T =&gt; For a given band, record and zoom, all the blocks will be contiguous.
- To optimize the access to timeseries of all the bands (such as in MUCOG): Z&gt;T&gt;R&gt;B =&gt; For a given zoom level and block, all the records will be contiguous.

Interlacing pattern can be specialized to only select a list or a range for each level (except Tile level).
- By values: L=0,2,3 will only select the value 0, 2 and 3 of the level L. For example B=0,2,3 to select the corresponding band level.
- By range: L=0:3 will only select the values from 0 to 3 (not included) of the level L. For example B=0:3 to select the three firsts bands. 
First and last values of the range can be omitted to define 0 or last element of the level. e.g B=2: means all the bands from the second.
Z=0 is the full resolution, Z=1 is the overview with zoom factor 2, Z=2 is the zoom factor 4, and so on.

To chain interlacing patterns, use &#34;;&#34; separator.

For example:
- MUCOG optimizes access to timeseries for full resolution (Z=0), but geographic for overviews (Z=1:). Z=0&gt;T&gt;R&gt;B;Z=1:&gt;R&gt;T&gt;B
- Same example, but the bands are separated: B&gt;Z=0&gt;T&gt;R;B&gt;Z=1:&gt;R&gt;T
- To optimize access to geographic information of the three first bands together, but timeseries of the others: Z&gt;T&gt;R&gt;B=0:3;B=3:&gt;Z&gt;R&gt;T


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| grid_flags | [string](#string) | repeated | External layout: Grid:Cell (CRS) |
| grid_parameters | [Layout.GridParametersEntry](#geocube-Layout-GridParametersEntry) | repeated |  |
| block_x_size | [int64](#int64) |  | Internal layout: Cell, Tile |
| block_y_size | [int64](#int64) |  |  |
| max_records | [int64](#int64) |  |  |
| overviews_min_size | [int64](#int64) |  | Maximum width or height of the smallest overview level. 0: No overview, -1: default=256. |
| interlacing_pattern | [string](#string) |  | Define how to interlace the [R]ecords, the [B]ands, the [Z]ooms level/overview and the [T]iles (geotiff blocks). |






<a name="geocube-Layout-GridParametersEntry"></a>

### Layout.GridParametersEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-ListGridsRequest"></a>

### ListGridsRequest
List all the grids given a name pattern (does not retrieve the cells)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name_like | [string](#string) |  | Name pattern (support * and ? for all or any characters and trailing (?i) for case-insensitiveness) |






<a name="geocube-ListGridsResponse"></a>

### ListGridsResponse
Return a list of grids


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| grids | [Grid](#geocube-Grid) | repeated |  |






<a name="geocube-ListLayoutsRequest"></a>

### ListLayoutsRequest
List all the layouts given a name pattern


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name_like | [string](#string) |  | Name pattern (support * and ? for all or any characters and trailing (?i) for case-insensitiveness) |






<a name="geocube-ListLayoutsResponse"></a>

### ListLayoutsResponse
Return a list of layouts


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| layouts | [Layout](#geocube-Layout) | repeated |  |






<a name="geocube-Size"></a>

### Size
Define a size


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| width | [int32](#int32) |  |  |
| height | [int32](#int32) |  |  |






<a name="geocube-Tile"></a>

### Tile
Define a rectangular tile in a given coordinate system (CRS).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| transform | [GeoTransform](#geocube-GeoTransform) |  | Transform to map from pixel coordinates to CRS |
| size_px | [Size](#geocube-Size) |  | Size of the tile in pixel |
| crs | [string](#string) |  | Coordinate reference system |






<a name="geocube-TileAOIRequest"></a>

### TileAOIRequest
Tile an AOI, covering it with cells defined by a grid.
In the future, it will be able to find the best tiling given the internal layout of datasets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| aoi | [AOI](#geocube-AOI) |  |  |
| layout_name | [string](#string) |  | Name of an existing layout |
| layout | [Layout](#geocube-Layout) |  | User-defined layout |






<a name="geocube-TileAOIResponse"></a>

### TileAOIResponse
Return tiles, thousand by thousand.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tiles | [Tile](#geocube-Tile) | repeated |  |





 

 

 

 



<a name="pb_operations-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/operations.proto



<a name="geocube-CancelJobRequest"></a>

### CancelJobRequest
Cancel a job (e.g. during consolidation)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| force_any_state | [bool](#bool) |  | Force cancel even when the job is not in a failed state or consolidation step (could corrupt the data) |






<a name="geocube-CancelJobResponse"></a>

### CancelJobResponse







<a name="geocube-CleanJobsRequest"></a>

### CleanJobsRequest
Clean terminated jobs


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name_like | [string](#string) |  | Filter by name (support *, ? and (?i)-suffix for case-insensitivity) |
| state | [string](#string) |  | Filter by terminated state (DONE, FAILED) |






<a name="geocube-CleanJobsResponse"></a>

### CleanJobsResponse
Return the number of jobs that have been deleted


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int32](#int32) |  |  |






<a name="geocube-ConfigConsolidationRequest"></a>

### ConfigConsolidationRequest
Configure the parameters of the consolidation attached to the variable


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable_id | [string](#string) |  |  |
| consolidation_params | [ConsolidationParams](#geocube-ConsolidationParams) |  |  |






<a name="geocube-ConfigConsolidationResponse"></a>

### ConfigConsolidationResponse







<a name="geocube-ConsolidateRequest"></a>

### ConsolidateRequest
Create and start a consolidation job given a list of records and an instance_id to be consolidated on a layout
Optionnaly, the job can be done step by step, pausing and waiting for user action, with three levels:
- 1: after each critical steps
- 2: after each major steps
- 3: after all steps


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| job_name | [string](#string) |  |  |
| instance_id | [string](#string) |  |  |
| layout_name | [string](#string) |  |  |
| execution_level | [ExecutionLevel](#geocube-ExecutionLevel) |  | Execution level of a job. A consolidation job cannot be executed synchronously |
| collapse_on_record_id | [string](#string) |  | [Optional] Collapse all records on this record (in this case only, original datasets are kept, data is duplicated) |
| records | [RecordIdList](#geocube-RecordIdList) |  | At least one |
| filters | [RecordFilters](#geocube-RecordFilters) |  |  |






<a name="geocube-ConsolidateResponse"></a>

### ConsolidateResponse
Return the id of the job created


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| job_id | [string](#string) |  |  |






<a name="geocube-ConsolidationParams"></a>

### ConsolidationParams
Parameters of consolidation that are linked to a variable, to define:
- how to resample the data during consolidation
- how to store the data:
  - Compression
  - CreationParams (supported: see GDAL drivers: PHOTOMETRIC, COMPRESS, PREDICTOR, ZLEVEL, ZSTDLEVEL, MAX_Z_ERROR, JPEGTABLESMODE and with _OVERVIEW suffix if exists)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dformat | [DataFormat](#geocube-DataFormat) |  | dataformat of the data. See exponent for the mapping formula. |
| exponent | [double](#double) |  | 1: linear scaling (RealMax - RealMin) * pow( (Value - Min) / (Max - Min), Exponent) &#43; RealMin |
| create_overviews | [bool](#bool) |  | **Deprecated.** Use Layout.overviews_min_size instead |
| resampling_alg | [Resampling](#geocube-Resampling) |  | Define how to resample the data during the consolidation (if a reprojection is needed or if the overviews are created) |
| compression | [ConsolidationParams.Compression](#geocube-ConsolidationParams-Compression) |  | Define how the data is compressed at block level |
| creation_params | [ConsolidationParams.CreationParamsEntry](#geocube-ConsolidationParams-CreationParamsEntry) | repeated | map of params:value to configure the creation of the file. See Compression to list the supported params |
| bands_interleave | [bool](#bool) |  | **Deprecated.** If the variable is multibands, define whether the bands are interleaved. Use Layout.interlacing_pattern instead |
| storage_class | [StorageClass](#geocube-StorageClass) |  | Define the storage class of the created file (support only GCS) |






<a name="geocube-ConsolidationParams-CreationParamsEntry"></a>

### ConsolidationParams.CreationParamsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="geocube-Container"></a>

### Container
Define a container of datasets.
Usually a container is a file containing one dataset.
But after a consolidation or if the container has several bands, it can contain several datasets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uri | [string](#string) |  | URI of the file |
| managed | [bool](#bool) |  | True if the Geocube is responsible for the lifecycle of this file |
| datasets | [Dataset](#geocube-Dataset) | repeated | List of datasets of the container |






<a name="geocube-ContinueJobRequest"></a>

### ContinueJobRequest
Proceed the next step of a step-by-step job


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |






<a name="geocube-ContinueJobResponse"></a>

### ContinueJobResponse







<a name="geocube-Dataset"></a>

### Dataset
Define a dataset. A dataset is the metadata to retrieve an image from a file.
It is defined by a record and the instance of a variable.

A dataset defines:
- Which band(s) are indexed (usually all the bands, but it can be a subset)
- How to map the value of its pixels to the dataformat of the variable. In more details:
   . the dataformat of the dataset (dformat.[no_data, min, max]) that describes the pixel of the image
   . the mapping from each pixel to the data format of the variable (variable.dformat). This mapping is defined as [MinOut, MaxOut, Exponent].


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| record_id | [string](#string) |  |  |
| instance_id | [string](#string) |  |  |
| container_subdir | [string](#string) |  |  |
| bands | [int64](#int64) | repeated |  |
| dformat | [DataFormat](#geocube-DataFormat) |  | Internal data format (DType can be Undefined) |
| real_min_value | [double](#double) |  | Real min value (dformat.min_value maps to real_min_value) |
| real_max_value | [double](#double) |  | Real max value (dformat.max_value maps to real_max_value) |
| exponent | [double](#double) |  | 1: linear scaling (RealMax - RealMin) * pow( (Value - Min) / (Max - Min), Exponent) &#43; RealMin |






<a name="geocube-GetConsolidationParamsRequest"></a>

### GetConsolidationParamsRequest
Retrieve the consolidation parameters attached to the given variable


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| variable_id | [string](#string) |  |  |






<a name="geocube-GetConsolidationParamsResponse"></a>

### GetConsolidationParamsResponse
Return consolidation parameters


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| consolidation_params | [ConsolidationParams](#geocube-ConsolidationParams) |  |  |






<a name="geocube-GetContainersRequest"></a>

### GetContainersRequest
Request info on containers


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uris | [string](#string) | repeated | List of container uris |






<a name="geocube-GetContainersResponse"></a>

### GetContainersResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| containers | [Container](#geocube-Container) | repeated |  |






<a name="geocube-GetJobRequest"></a>

### GetJobRequest
Retrieve a job given its id


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| log_page | [int32](#int32) |  |  |
| log_limit | [int32](#int32) |  |  |






<a name="geocube-GetJobResponse"></a>

### GetJobResponse
Return a job with the requested id


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| job | [Job](#geocube-Job) |  |  |






<a name="geocube-IndexDatasetsRequest"></a>

### IndexDatasetsRequest
Request to index all the datasets of a container


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container | [Container](#geocube-Container) |  | TODO Index several containers: repeated ? |






<a name="geocube-IndexDatasetsResponse"></a>

### IndexDatasetsResponse







<a name="geocube-Job"></a>

### Job
Job to modify datasets (consolidation, deletion, ingestion...)
Some lifecycle operations are required to be done cautiously, in order to garantee the integrity of the database.
Such operations are defined by a job and are done asynchronously.
A job is a state-machine that can be rollbacked anytime during the operation until it ends.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Id of the job |
| name | [string](#string) |  | Name of the job (must be unique) |
| type | [string](#string) |  | Type of the job (consolidation, deletion...) |
| state | [string](#string) |  | Current state of the state machine |
| creation_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Time of creation of the job |
| last_update_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Time of the last update |
| logs | [string](#string) | repeated | Job logs: if logs are too big to fit in a grpc response, logs will only be a subset (by default, the latest) |
| active_tasks | [int32](#int32) |  | If the job is divided into sub tasks, number of pending tasks |
| failed_tasks | [int32](#int32) |  | If the job is divided into sub tasks, number of failed tasks |
| execution_level | [ExecutionLevel](#geocube-ExecutionLevel) |  | Execution level of a job (see ExecutionLevel) |
| waiting | [bool](#bool) |  | If true, the job is waiting for user to continue |






<a name="geocube-ListJobsRequest"></a>

### ListJobsRequest
List jobs given a name pattern


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name_like | [string](#string) |  |  |
| page | [int32](#int32) |  |  |
| limit | [int32](#int32) |  |  |






<a name="geocube-ListJobsResponse"></a>

### ListJobsResponse
Return a list of the job whose name matchs the pattern


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| jobs | [Job](#geocube-Job) | repeated |  |






<a name="geocube-RetryJobRequest"></a>

### RetryJobRequest
Retry a job that failed or is stuck (e.g. during consolidation)


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| force_any_state | [bool](#bool) |  | Force retry even when the job is not in a failed state (could corrupt the data) |






<a name="geocube-RetryJobResponse"></a>

### RetryJobResponse






 


<a name="geocube-ConsolidationParams-Compression"></a>

### ConsolidationParams.Compression


| Name | Number | Description |
| ---- | ------ | ----------- |
| NO | 0 |  |
| LOSSLESS | 1 |  |
| LOSSY | 2 |  |
| CUSTOM | 3 | configured by creation_params |



<a name="geocube-ExecutionLevel"></a>

### ExecutionLevel
Execution level of a job

| Name | Number | Description |
| ---- | ------ | ----------- |
| ExecutionSynchronous | 0 | Job is done synchronously |
| ExecutionAsynchronous | 1 | Job is done asynchronously, but without any pause |
| StepByStepCritical | 2 | Job is done asynchronously, step-by-step, pausing at every critical steps |
| StepByStepMajor | 3 | Job is done asynchronously, step-by-step, pausing at every major steps |
| StepByStepAll | 4 | Job is done asynchronously, step-by-step, pausing at every steps |



<a name="geocube-StorageClass"></a>

### StorageClass
Storage class of a container. Depends on the storage

| Name | Number | Description |
| ---- | ------ | ----------- |
| STANDARD | 0 |  |
| INFREQUENT | 1 |  |
| ARCHIVE | 2 |  |
| DEEPARCHIVE | 3 |  |


 

 

 



<a name="pb_datasetMeta-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/datasetMeta.proto



<a name="geocube-DatasetMeta"></a>

### DatasetMeta
DatasetMeta contains all the metadata on files and fileformats to download and generate a slice of a cube


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| internalsMeta | [InternalMeta](#geocube-InternalMeta) | repeated | Information on the images composing the slice |






<a name="geocube-InternalMeta"></a>

### InternalMeta
InternalMeta contains all the metadata on a file to download it and to map its internal values to the external range.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_uri | [string](#string) |  | URI of the file storing the data |
| container_subdir | [string](#string) |  | Subdir of the file storing the data |
| bands | [int64](#int64) | repeated | Subbands of the file requested |
| dformat | [DataFormat](#geocube-DataFormat) |  | Internal dataformat of the data |
| range_min | [double](#double) |  | dformat.RangeMin will be mapped to this value |
| range_max | [double](#double) |  | dformat.RangeMax will be mapped to this value |
| exponent | [double](#double) |  | Exponent used to map the value from dformat to [RangeMin, RangeMax] |





 

 

 

 



<a name="pb_version-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## pb/version.proto



<a name="geocube-GetVersionRequest"></a>

### GetVersionRequest
Request the version of the Geocube






<a name="geocube-GetVersionResponse"></a>

### GetVersionResponse
Return the version of the Geocube


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Version | [string](#string) |  |  |





 

 

 

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

