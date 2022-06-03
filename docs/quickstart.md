# Getting started

This quickstart requires [docker-compose](https://docs.docker.com/compose/install/) and `python3.8`.

- Clone `https://github.com/airbusgeo/geocube.git`.
- Start geocube service with docker-compose, following [these instructions](installation/docker-install.md#docker-compose).
- Install `geocube-client-python` in your prefered environment :
```shell 
pip install git+https://github.com/airbusgeo/geocube-client-python.git
```
- Start a python console and type
```python
import geocube
client = geocube.Client("127.0.0.1:8080")
```
You are connected !

Then, you can do the [tutorials](user-guide/tutorials.md) to learn how to feed the geocube, access and optimize the data.

