# Scaling-up

The Geocube is designed to handle very large processing flow.

## Tile an AOI

Usually, it's not possible to process all images of a large area in a row. The area has to be divided into smaller tiles for the memory to fit into the processing machine.

The Geocube provides a convenient way to tile an aoi, thanks to the [TileAOI()](grpc.md#tileaoirequest). It takes an AOI and a layout and streams a list of tiles.

## Using the downloader

With tiling, the process can be parallelized on processing workers, but a bottleneck can occur when it's time to get cubes of data from the Geocube Server. To prevent from that, either more Geocube Server machines must be provisioned or the processing workers can use a [local](../installation/local-install.md#downloader) or [dockerized](../installation/docker-install.md#run-downloader---examples) [downloader service](../architecture/services.md#downloader). For the latter case, the local machines should have an efficient access to the object storages (they should be on the same network) or the datasets should be consolidated with the same layout to limit the volume of transfered data.

## Using Dask

[Dask](https://www.dask.org) is a flexible open-source Python library for parallel computing. The [geocube-client-python](https://www.github.com/airbusgeo/geocube-client-python.git) provides several [examples](https://github.com/airbusgeo/geocube-client-python/blob/main/Jupyter/Geocube-Client-SDK-1.ipynb) and a [docker](https://github.com/airbusgeo/geocube-client-python/tree/main/docker) to use Dask with the Geocube and the Downloader services.
