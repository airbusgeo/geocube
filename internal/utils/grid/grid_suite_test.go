package grid_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGrid(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Grid Suite")
}
