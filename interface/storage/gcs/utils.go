package gcs

import (
	"fmt"
	"strings"
)

//Parse takes in a string in the form gs://bucket/path/to/object or
// bucket/path/to/object or /bucket/path/to/object and returns the
// bucket and object strings as usable by the cloud.google.com/storage
// Client
func Parse(gsUri string) (bucket, object string, err error) {
	bucket, object = parse(gsUri)
	if len(bucket) == 0 || len(object) == 0 {
		err = fmt.Errorf("missing bucket or object")
	}
	return
}

func parse(gsUri string) (bucket, object string) {
	if strings.HasPrefix(gsUri, "gs://") {
		gsUri = strings.TrimPrefix(gsUri, "gs://")
	} else {
		gsUri = strings.TrimPrefix(gsUri, "/")
	}
	firstSlash := strings.Index(gsUri, "/")
	if firstSlash == -1 {
		bucket = gsUri
		object = ""
	} else {
		bucket = gsUri[0:firstSlash]
		object = gsUri[firstSlash+1:]
	}
	return
}
