package geocube

//go:generate protoc -I api/v1/ --go_opt=paths=source_relative --go_out=plugins=grpc:internal --grpc-gateway_out=logtostderr=true:internal pb/version.proto pb/geocube.proto pb/catalog.proto pb/records.proto pb/dataformat.proto pb/variables.proto pb/layouts.proto pb/operations.proto pb/datasetMeta.proto pb/geocubeDownloader.proto
//go:generate protoc -I api/v1/ --go_opt=paths=source_relative --go_out=plugins=grpc:internal pb/admin.proto
