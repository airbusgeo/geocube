package mucog

import (
	"fmt"

	"github.com/google/tiff"
	"github.com/google/tiff/bigtiff"
)

func LoadTIFF(tif tiff.TIFF) ([]*IFD, error) {
	ntop := 0
	nLegacyIFDs := 0 //number of top level ifds that are actually an overview or a mask
	topidx := -1
	for i, ifd := range tif.IFDs() {
		if !ifd.HasField(254) {
			ntop++
			topidx = i
		} else {
			nLegacyIFDs++
		}
	}
	//we support multiple top level ifds provided there are no legacy ones
	if ntop == 0 ||
		(ntop > 1 && nLegacyIFDs > 0) {
		return nil, fmt.Errorf("unsupported combination of top level/legacy IFDs: %d/%d", ntop, nLegacyIFDs)
	}
	mifds := []*IFD{}
	isbigtiff := tif.Version() == bigtiff.Version
	if nLegacyIFDs == 0 {
		for _, ifd := range tif.IFDs() {
			mifd, err := loadIFD(tif.R(), ifd, isbigtiff)
			if err != nil {
				return nil, err
			}
			mifds = append(mifds, mifd)
		}
	} else {
		mifd, err := loadIFD(tif.R(), tif.IFDs()[topidx], isbigtiff)
		if err != nil {
			return nil, err
		}
		for i, ifd := range tif.IFDs() {
			if i == topidx {
				continue
			}
			sifd, err := loadIFD(tif.R(), ifd, isbigtiff)
			if err != nil {
				return nil, err
			}
			mifd.SubIFDs = append(mifd.SubIFDs, sifd)
			mifd.SubIFDOffsets = append(mifd.SubIFDOffsets, 0)
		}
		mifds = append(mifds, mifd)
	}
	return mifds, nil
}

func loadIFD(r tiff.BReader, tifd tiff.IFD, isbigtiff bool) (*IFD, error) {
	err := sanityCheckIFD(tifd)
	if err != nil {
		return nil, err
	}
	ifd := &IFD{r: r}
	err = tiff.UnmarshalIFD(tifd, ifd)
	if err != nil {
		return nil, err
	}
	if len(ifd.TempTileByteCounts) > 0 {
		ifd.TileByteCounts = make([]uint32, len(ifd.TempTileByteCounts))
		for i := range ifd.TempTileByteCounts {
			ifd.TileByteCounts[i] = uint32(ifd.TempTileByteCounts[i])
		}
		ifd.TempTileByteCounts = nil //reclaim mem
	}
	if len(ifd.SubIFDOffsets) > 0 {
		ifd.SubIFDs = make([]*IFD, len(ifd.SubIFDOffsets))
		for s, soff := range ifd.SubIFDOffsets {
			ifd.SubIFDs[s], err = loadOffset(r, soff, isbigtiff)
			if err != nil {
				return nil, fmt.Errorf("load offset %d/%d: %w", soff, s, err)
			}
			if ifd.SubIFDs[s].SubfileType == 0 {
				//log.Printf("%+v", ifd.SubIFDs[s])
				return nil, fmt.Errorf("subifd %d: cannot use subIFD with no subfiletype set", s)
			}
		}
	}
	return ifd, nil
}

func loadOffset(r tiff.BReader, off uint64, isbigtiff bool) (*IFD, error) {
	var ifd tiff.IFD
	var err error
	if isbigtiff {
		ifd, err = bigtiff.ParseIFD(r, off, nil, nil)
	} else {
		ifd, err = tiff.ParseIFD(r, off, nil, nil)
	}
	if err != nil {
		return nil, err
	}
	return loadIFD(r, ifd, isbigtiff)
}

func sanityCheckIFD(ifd tiff.IFD) error {
	to := ifd.GetField(324)
	tl := ifd.GetField(325)
	if to == nil || tl == nil {
		return fmt.Errorf("no tiles")
	}
	if to.Count() != tl.Count() {
		return fmt.Errorf("inconsistent tile off/len count")
	}
	so := ifd.GetField(272)
	sl := ifd.GetField(279)
	if so != nil || sl != nil {
		return fmt.Errorf("tif has strips")
	}
	return nil
}
