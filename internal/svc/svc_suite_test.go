package svc_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSVC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SVC Suite")
}
