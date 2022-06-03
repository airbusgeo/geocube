package geocube

//go:generate docker run --rm -v $PWD/api/v1:/protos -v $PWD/docs/user-guide:/out pseudomuto/protoc-gen-doc -I /protos --doc_opt=markdown,grpc.md pb/geocube.proto pb/geocubeDownloader.proto pb/admin.proto pb/records.proto pb/variables.proto pb/dataformat.proto pb/catalog.proto pb/layouts.proto pb/operations.proto pb/datasetMeta.proto pb/version.proto
