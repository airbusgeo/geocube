package utils

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// F64ToS converts float to string using the maximum accuracy
func F64ToS(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// AverageElemF computes the mean value of vs.$
// AverageElemF panics if len(vs) = 0
func AverageElemF(vs []float64) float64 {
	var va float64 = 0
	for _, v := range vs {
		va += v
	}
	return va / float64(len(vs))
}

// MinElemF computes the min value of vs.
// MinElemF panics if len(vs) = 0
func MinElemF(vs []float64) float64 {
	vm := vs[0]
	for _, v := range vs {
		if v < vm {
			vm = v
		}
	}
	return vm
}

// MaxElemF computes the meax value of vs
// MaxElemF panics if len(vs)=0
func MaxElemF(vs []float64) float64 {
	vm := vs[0]
	for _, v := range vs {
		if v > vm {
			vm = v
		}
	}
	return vm
}

// MinI computes the min value between two integers
func MinI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxI computes the max value between two integers
func MaxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

/*
FindRegexGroups returns a map containing the group names as keys and the values matched as values, if the string value matches the regex.
*/
func FindRegexGroups(reg *regexp.Regexp, v string) (map[string]string, error) {
	matches := reg.FindStringSubmatch(v)
	if len(matches) == 0 {
		return nil, fmt.Errorf("failed to find submatch in regex %v for value %v", reg.String(), v)
	}

	groupNames := reg.SubexpNames()
	matches, groupNames = matches[1:], groupNames[1:]
	res := make(map[string]string, len(matches))
	for i := range groupNames {
		res[groupNames[i]] = matches[i]
	}

	return res, nil
}

func URLJoin(url string, elems ...string) string {
	return fmt.Sprintf("%s/%s", strings.TrimRight(url, "/"), path.Join(elems...))
}
