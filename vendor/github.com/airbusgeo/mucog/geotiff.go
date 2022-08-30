package mucog

import (
	"errors"
	"fmt"
)

type geotransform [6]float64

func (gt geotransform) Origin() (float64, float64) {
	return gt[0], gt[3]
}

func (gt geotransform) Scale() (float64, float64) {
	return gt[1], gt[5]
}

func (gt geotransform) Transform(x, y float64) (float64, float64) {
	return gt[0] + gt[1]*x + gt[2]*y, gt[3] + gt[4]*x + gt[5]*y
}

func (gt geotransform) Inverse() (geotransform, error) {
	idet := 1.0 / (gt[1]*gt[5] - gt[2]*gt[4])
	if idet == 0 {
		return geotransform{}, fmt.Errorf("non invertible geotransform %v", gt)
	}
	res := geotransform{0, gt[5] * idet, -gt[2] * idet, 0, -gt[4] * idet, gt[1] * idet}
	res[0], res[3] = res.Transform(-gt[0], -gt[3])
	return res, nil
}

func (ifd *IFD) geotransform() (geotransform, error) {
	//TODO: check and return error if geotiff ix PIXELISPOINT
	gt := geotransform{0, 1, 0, 0, 0, 1}
	if len(ifd.ModelPixelScaleTag) >= 2 &&
		ifd.ModelPixelScaleTag[0] != 0 && ifd.ModelPixelScaleTag[1] != 0 {
		gt[1] = ifd.ModelPixelScaleTag[0]
		gt[5] = -ifd.ModelPixelScaleTag[1]

		if len(ifd.ModelTiePointTag) >= 6 {
			gt[0] =
				ifd.ModelTiePointTag[3] -
					ifd.ModelTiePointTag[0]*gt[1]
			gt[3] =
				ifd.ModelTiePointTag[4] -
					ifd.ModelTiePointTag[1]*gt[5]
		}
	} else if len(ifd.ModelTransformationTag) == 16 {
		gt[0] = ifd.ModelTransformationTag[3]
		gt[1] = ifd.ModelTransformationTag[0]
		gt[2] = ifd.ModelTransformationTag[1]
		gt[3] = ifd.ModelTransformationTag[7]
		gt[4] = ifd.ModelTransformationTag[4]
		gt[5] = ifd.ModelTransformationTag[5]
	} else {
		return gt, errors.New("no geotiff referencing computed")
	}
	return gt, nil
}
