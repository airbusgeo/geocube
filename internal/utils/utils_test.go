package utils_test

import (
	"fmt"

	"github.com/airbusgeo/geocube/internal/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Temporary error", func() {
	var err error

	var (
		itShouldReturnATemporaryError = func() {
			It("it should return a temporary error", func() {
				Expect(utils.Temporary(err)).To(BeTrue())
			})
		}
		itShouldReturnAPermanentError = func() {
			It("it should return a permanent error", func() {
				Expect(utils.Temporary(err)).To(BeFalse())
			})
		}
	)

	Describe("Temporary", func() {
		JustBeforeEach(func() {
			err = fmt.Errorf("temporary err :%w", utils.MakeTemporary(fmt.Errorf("Temporary")))
		})

		Context("Return temporary", func() {
			itShouldReturnATemporaryError()
		})
	})

	Describe("Permanent", func() {
		JustBeforeEach(func() {
			err = fmt.Errorf("permanent err :%v", utils.MakeTemporary(fmt.Errorf("Temporary")))
		})

		Context("Return permanent", func() {
			itShouldReturnAPermanentError()
		})
	})
})
