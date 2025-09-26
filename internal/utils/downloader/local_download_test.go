package downloader_test

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/airbusgeo/geocube/internal/utils/downloader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalDownloader", func() {
	var (
		// args
		ctx          = context.Background()
		remoteFolder string
		localFolder  string
		remoteFile1  string
		remoteFile2  string

		returnedError error
		filesInfo     map[string]*downloader.LocalFileInfo

		localDownloader *downloader.LocalDownloader
		returnedURI     string
	)

	BeforeEach(func() {
		remoteFolder = path.Join(os.TempDir(), "remote")
		localFolder = path.Join(os.TempDir(), "local")
		err := os.MkdirAll(remoteFolder, os.ModePerm)
		Expect(err).To(BeNil())
		err = os.MkdirAll(localFolder, os.ModePerm)
		Expect(err).To(BeNil())

		remoteFile1 = path.Join(remoteFolder, "test1")
		remoteFile2 = path.Join(remoteFolder, "test2")
		err = os.WriteFile(remoteFile1, []byte("0000000000"), 0666)
		Expect(err).To(BeNil())
		err = os.WriteFile(remoteFile2, []byte("0000000000"), 0666)
		Expect(err).To(BeNil())
	})
	AfterEach(func() {
		os.Remove(remoteFile1)
		os.Remove(remoteFile2)
	})

	var (
		itShouldNotReturnAnError = func() {
			It("it should not return an error", func() {
				Expect(returnedError).To(BeNil())
			})
		}

		itShouldReturnLocalFile = func() {
			It("it should create local file", func() {
				Expect(returnedURI).To(ContainSubstring(localFolder))
			})
		}

		itShouldCreateLocalFile = func() {
			It("it should create local file", func() {
				_, err := os.Stat(returnedURI)
				Expect(err).To(BeNil())
			})
		}
		itShouldNotCreateLocalFile = func() {
			It("it should not create local file", func() {
				Expect(returnedURI).NotTo(ContainSubstring(localFolder))
			})
		}

		itShouldRemoveURI = func() {
			It("it should remove URI", func() {
				time.Sleep(time.Millisecond * 500)
				_, err := os.Stat(returnedURI)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		}
	)

	Context("download one remote dataset", func() {
		BeforeEach(func() {
			filesInfo = map[string]*downloader.LocalFileInfo{
				remoteFile1: {ReferenceNb: 1, Size: 10},
				remoteFile2: {ReferenceNb: 1, Size: 10},
			}

			localDownloader = downloader.NewLocalDownloader(ctx, filesInfo, localFolder, 10, 5)
			returnedURI = localDownloader.GetURI(remoteFile1)
		})
		AfterEach(func() {
			localDownloader.ReleaseURI(ctx, remoteFile1)
		})
		itShouldNotReturnAnError()
		itShouldCreateLocalFile()
	})

	Context("download and release one remote dataset", func() {
		BeforeEach(func() {
			filesInfo = map[string]*downloader.LocalFileInfo{
				remoteFile1: {ReferenceNb: 3, Size: 10},
				remoteFile2: {ReferenceNb: 1, Size: 10},
			}

			localDownloader = downloader.NewLocalDownloader(ctx, filesInfo, localFolder, 10, 5)
			returnedURI = localDownloader.GetURI(remoteFile1)
			localDownloader.GetURI(remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
			localDownloader.GetURI(remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
		})
		AfterEach(func() {
		})
		itShouldNotReturnAnError()
		itShouldReturnLocalFile()
		itShouldRemoveURI()
	})
	Context("download two remote datasets", func() {
		BeforeEach(func() {
			filesInfo = map[string]*downloader.LocalFileInfo{
				remoteFile1: {ReferenceNb: 1, Size: 10},
				remoteFile2: {ReferenceNb: 1, Size: 10},
			}

			localDownloader = downloader.NewLocalDownloader(ctx, filesInfo, localFolder, 10, 5)
			localDownloader.GetURI(remoteFile1)
			returnedURI = localDownloader.GetURI(remoteFile2)
		})
		AfterEach(func() {
			localDownloader.ReleaseURI(ctx, remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile2)
		})
		itShouldNotReturnAnError()
		itShouldNotCreateLocalFile()
	})
	Context("download three remote datasets", func() {
		BeforeEach(func() {
			filesInfo = map[string]*downloader.LocalFileInfo{
				remoteFile1: {ReferenceNb: 2, Size: 10},
				remoteFile2: {ReferenceNb: 1, Size: 10},
			}

			localDownloader = downloader.NewLocalDownloader(ctx, filesInfo, localFolder, 10, 5)
			localDownloader.GetURI(remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
			returnedURI = localDownloader.GetURI(remoteFile2)
			localDownloader.GetURI(remoteFile1)
		})
		AfterEach(func() {
			localDownloader.ReleaseURI(ctx, remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile2)
		})
		itShouldNotReturnAnError()
		itShouldNotCreateLocalFile()
	})
	Context("download three remote datasets one by one", func() {
		BeforeEach(func() {
			filesInfo = map[string]*downloader.LocalFileInfo{
				remoteFile1: {ReferenceNb: 2, Size: 10},
				remoteFile2: {ReferenceNb: 1, Size: 10},
			}

			localDownloader = downloader.NewLocalDownloader(ctx, filesInfo, localFolder, 10, 5)
			localDownloader.GetURI(remoteFile1)
			localDownloader.GetURI(remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
			localDownloader.ReleaseURI(ctx, remoteFile1)
			returnedURI = localDownloader.GetURI(remoteFile2)
		})
		AfterEach(func() {
			localDownloader.ReleaseURI(ctx, remoteFile2)
		})
		itShouldNotReturnAnError()
		itShouldCreateLocalFile()
	})
})
