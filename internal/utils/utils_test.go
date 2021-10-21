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

var _ = Describe("Merge error", func() {
	var err error

	var tmpErr = utils.MakeTemporary(fmt.Errorf("Temporary"))
	var fatalErr = fmt.Errorf("Fatal")

	var (
		itShouldReturnNil = func() {
			It("it should return nil", func() {
				Expect(err).To(BeNil())
			})
		}
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

	Describe("nil then err", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(false, nil, fatalErr)
		})

		Context("Return temporary", func() {
			It("it should return an error", func() {
				Expect(err).To(Equal(fatalErr))
			})
		})
	})

	Describe("Temporary then fatal, priority to temporary", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(false, tmpErr, fatalErr)
		})

		Context("Return temporary", func() {
			itShouldReturnATemporaryError()
		})
	})

	Describe("Temporary then fatal then nil, priority to temporary", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(false, tmpErr, fatalErr, nil)
		})

		Context("Return nil", func() {
			itShouldReturnNil()
		})
	})

	Describe("Temporary then fatal then nil, priority to fatal", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(true, tmpErr, fatalErr, nil)
		})

		Context("Return fatal", func() {
			itShouldReturnAPermanentError()
		})
	})

	Describe("Fatal then temporary, priority to temporary", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(false, fatalErr, tmpErr)
		})

		Context("Return temporary", func() {
			itShouldReturnATemporaryError()
		})
	})

	Describe("Fatal then temporary then nil, priority to temporary", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(false, fatalErr, tmpErr, nil)
		})

		Context("Return nil", func() {
			itShouldReturnNil()
		})
	})

	Describe("Fatal then temporary then nil, priority to fatal", func() {
		JustBeforeEach(func() {
			err = utils.MergeErrors(true, fatalErr, tmpErr, nil)
		})

		Context("Return fatal", func() {
			itShouldReturnAPermanentError()
		})
	})
})

var _ = Describe("URLJoin", func() {
	var path string

	var itShouldEqual = func(expectedPath string) {
		It("it should equal", func() {
			Expect(path).To(Equal(expectedPath))
		})
	}
	Describe("gs path", func() {
		JustBeforeEach(func() {
			path = utils.URLJoin("gs://my_bucket", "blob1", "blob2")
		})

		Context("", func() {
			itShouldEqual("gs://my_bucket/blob1/blob2")
		})
	})

	Describe("path with /", func() {
		JustBeforeEach(func() {
			path = utils.URLJoin("gs://my_bucket/", "blob1", "blob2")
		})

		Context("", func() {
			itShouldEqual("gs://my_bucket/blob1/blob2")
		})
	})

	Describe("local path with /", func() {
		JustBeforeEach(func() {
			path = utils.URLJoin("my_folder/", "blob1", "blob2")
		})

		Context("", func() {
			itShouldEqual("my_folder/blob1/blob2")
		})
	})

	Describe("local path with /", func() {
		JustBeforeEach(func() {
			path = utils.URLJoin("/my_folder/", "blob1", "blob2")
		})

		Context("", func() {
			itShouldEqual("/my_folder/blob1/blob2")
		})
	})
})
