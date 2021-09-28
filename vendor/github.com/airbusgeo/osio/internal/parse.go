package internal

import (
	"fmt"
	"strings"
)

func isSheme(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && r != '.' && r != '+' && r != '-' {
			return false
		}
	}
	return true
}

func BucketObject(input string) (string, string, error) {
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
