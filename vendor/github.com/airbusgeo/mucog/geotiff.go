package mucog

import "errors"

type geotransform [6]float64

func (gt geotransform) Origin() (float64, float64) {
	return gt[0], gt[3]
}

func (gt geotransform) Scale() (float64, float64) {
	return gt[1], -gt[5]
}

func (ifd *IFD) geotransform() (geotransform, error) {
	//TODO: check and return error if geotiff ix PIXELISPOINT
	gt := geotransform{0, 1, 0, 0, 0, 1}
	if len(ifd.ModelPixelScaleTag) >= 2 &&
		ifd.ModelPixelScaleTag[0] != 0 && ifd.ModelPixelScaleTag[1] != 0 {
		gt[1] = ifd.ModelPixelScaleTag[0]
		gt[5] = -ifd.ModelPixelScaleTag[1]
		if gt[5] > 0 {
			return gt, errors.New("negativ y-scale not supported")
		}

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
