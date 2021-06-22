package image

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/airbusgeo/mucog"

	"github.com/google/tiff"
)

type MucogGenerator interface {
	Create(workDir string, cogListFile []string) (string, error)
}

func NewMucogGenerator() MucogGenerator {
	return &mucogGenerator{}
}

type mucogGenerator struct{}

type cogFileInfo struct {
	Path    string
	Content *os.File
}

func (m *mucogGenerator) Create(workDir string, cogListFile []string) (string, error) {
	totalSize := int64(0)
	multicog := mucog.New()
	for _, cogFilePath := range cogListFile {
		cogFile, err := os.Open(cogFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to open cog file: %w", err)
		}

		//noinspection GoDeferInLoop
		defer cogFile.Close()

		totalSize, err = m.addCog(multicog, totalSize, cogFileInfo{Content: cogFile, Path: cogFilePath})
		if err != nil {
			return "", fmt.Errorf("failed to append multicog: %w", err)
		}
	}

	return m.writeMucog(multicog, workDir, totalSize)
}

func (m *mucogGenerator) addCog(multicog *mucog.MultiCOG, totalSize int64, c cogFileInfo) (int64, error) {
	st, err := c.Content.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", c.Content.Name(), err)
	}
	totalSize += st.Size()

	tiff, err := tiff.Parse(c.Content, nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to parse tiff file: %w", err)
	}

	tiffIFD, err := mucog.LoadTIFF(tiff)
	if err != nil {
		return 0, fmt.Errorf("failed to load tiff file: %w", err)
	}

	if len(tiffIFD) == 1 && tiffIFD[0].DocumentName == "" {
		tiffIFD[0].DocumentName = path.Base(c.Path)
		tiffIFD[0].DocumentName = strings.TrimSuffix(
			tiffIFD[0].DocumentName, filepath.Ext(tiffIFD[0].DocumentName))
	}

	for _, mifd := range tiffIFD {
		multicog.AppendIFD(mifd)
	}

	return totalSize, nil
}

func (m *mucogGenerator) writeMucog(multicog *mucog.MultiCOG, workDir string, totalSize int64) (string, error) {
	bigtiff := totalSize > int64(^uint32(0))
	mucogFilePath := path.Join(workDir, "mucog.tif")
	mucogFile, err := os.Create(mucogFilePath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", mucogFilePath, err)
	}

	if err = multicog.Write(mucogFile, bigtiff); err != nil {
		return "", fmt.Errorf("failed to write mucog: %w", err)
	}

	if err = mucogFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close mucog: %w", err)
	}
	return mucogFilePath, nil
}
