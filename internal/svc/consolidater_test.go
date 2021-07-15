package svc_test

import (
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/svc"
	"github.com/airbusgeo/godal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Consolidater", func() {

	var (
		datasetsToUse  []*svc.CsldDataset
		containerToUse geocube.ConsolidationContainer

		returnedNeed     bool
		returnedDatasets []string
	)

	BeforeEach(func() {
		godal.RegisterAll()
	})

	var (
		itShouldNeedConsolidation = func() {
			It("it should need consolidation", func() {
				Expect(returnedNeed).To(BeTrue())
			})
		}
		itShouldNotNeedConsolidation = func() {
			It("it should not need consolidation", func() {
				Expect(returnedNeed).To(BeFalse())
			})
		}
		itShouldNotReturnDatasets = func() {
			It("it should not return a dataset", func() {
				Expect(returnedDatasets).To(BeEmpty())
			})
		}
		itShouldReturnDataset = func(id string) {
			It("it should return a dataset", func() {
				Expect(returnedDatasets).To(Equal([]string{id}))
			})
		}
		itShouldReturnNDatasets = func(n int) {
			It("it should return N datasets", func() {
				Expect(len(returnedDatasets)).To(Equal(n))
			})
		}
	)

	Describe("CsldPrepareOrdersNeedReconsolidation", func() {

		JustBeforeEach(func() {
			returnedNeed, returnedDatasets = svc.CsldPrepareOrdersNeedReconsolidation(&datasetsToUse, &containerToUse)
		})

		Context("one basic dataset", func() {
			BeforeEach(func() {
				datasetsToUse = datasetNotConsolidated
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with no overview", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_123_NO
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with another dataformat", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedI_123_O
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset with other bands", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_234_O
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldNotReturnDatasets()
		})

		Context("one identical consolidated dataset", func() {
			BeforeEach(func() {
				datasetsToUse = datasetConsolidatedF_123_O
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnDataset(datasetConsolidatedF_123_O[0].Event.URI)
		})

		Context("several identical consolidated datasets", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnDataset(datasetsConsolidatedF_123_O[0].Event.URI)
		})

		Context("several identical consolidated datasets in two containers", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O_2
				containerToUse = containerF_3_O
			})
			itShouldNotNeedConsolidation()
			itShouldReturnNDatasets(2)
		})

		Context("several identical and different consolidated datasets in two containers", func() {
			BeforeEach(func() {
				datasetsToUse = datasetsConsolidatedF_123_O_3
				containerToUse = containerF_3_O
			})
			itShouldNeedConsolidation()
			itShouldReturnNDatasets(2)
		})
	})
})
