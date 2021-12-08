package gcs

import (
	"fmt"
	"io"
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

func bucketObject(input string) (string, string, error) {
	schemeIdx := strings.Index(input, "://")
	if schemeIdx >= 0 && isSheme(input[:schemeIdx]) {
		input = input[schemeIdx+3:]
	}
	skipSlash := 0
	for skipSlash = range input {
		if input[skipSlash] != '/' {
			break
		}
	}
	input = input[skipSlash:]
	sep := strings.Index(input, "/")
	if sep == -1 ||
		sep == len(input)-1 {
		return "", "", fmt.Errorf("not a bucket/object string")
	}
	return input[:sep], input[sep+1:], nil
}

func isSheme(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && r != '.' && r != '+' && r != '-' {
			return false
		}
	}
	return true
}

type readWrapper struct {
	io.ReadCloser
}

func addTemporaryCheck(err error) error {
	if err == nil || err == io.EOF || err == io.ErrUnexpectedEOF {
		return err
	}
	type terr interface {
		Temporary() bool
	}
	if _, ok := err.(terr); ok {
		return err
	}
	return &werr{err}
}

type werr struct {
	error
}
