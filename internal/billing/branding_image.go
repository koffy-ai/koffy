package billing

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
)

const (
	logoWidth           = 702
	logoHeight          = 180
	logoMaxBytes        = 200 * 1024
	logoMaxInputSize    = 5 * 1024 * 1024
	faviconSize         = 128
	faviconMaxBytes     = 100 * 1024
	faviconMaxInputSize = 2 * 1024 * 1024
	avatarSize          = 256
	avatarMaxBytes      = 200 * 1024
	avatarMaxInputSize  = 20 * 1024 * 1024
)

func processLogoUpload(raw []byte, filename string) (BrandingAsset, error) {
	if len(raw) == 0 {
		return BrandingAsset{}, fmt.Errorf("uploaded file is empty")
	}
	if len(raw) > logoMaxInputSize {
		return BrandingAsset{}, fmt.Errorf("uploaded file must be smaller than 5MB")
	}

	if cfg, err := png.DecodeConfig(bytes.NewReader(raw)); err == nil &&
		cfg.Width == logoWidth &&
		cfg.Height == logoHeight &&
		len(raw) < logoMaxBytes {
		return BrandingAsset{
			ContentType:      "image/png",
			Data:             raw,
			SizeBytes:        len(raw),
			Width:            logoWidth,
			Height:           logoHeight,
			OriginalFilename: filename,
		}, nil
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return BrandingAsset{}, fmt.Errorf("unsupported image file")
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, logoWidth, logoHeight))
	draw.Draw(canvas, canvas.Bounds(), image.Transparent, image.Point{}, draw.Src)

	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return BrandingAsset{}, fmt.Errorf("invalid image size")
	}

	scale := math.Min(float64(logoWidth)/float64(srcW), float64(logoHeight)/float64(srcH))
	dstW := max(1, int(math.Round(float64(srcW)*scale)))
	dstH := max(1, int(math.Round(float64(srcH)*scale)))
	offsetX := (logoWidth - dstW) / 2
	offsetY := (logoHeight - dstH) / 2
	scaleNearest(canvas, img, image.Rect(offsetX, offsetY, offsetX+dstW, offsetY+dstH))

	data, err := encodePNGUnderLimit(canvas)
	if err != nil {
		return BrandingAsset{}, err
	}
	return BrandingAsset{
		ContentType:      "image/png",
		Data:             data,
		SizeBytes:        len(data),
		Width:            logoWidth,
		Height:           logoHeight,
		OriginalFilename: filename,
	}, nil
}

func processFaviconUpload(raw []byte, filename string) (BrandingAsset, error) {
	if len(raw) == 0 {
		return BrandingAsset{}, fmt.Errorf("uploaded file is empty")
	}
	if len(raw) > faviconMaxInputSize {
		return BrandingAsset{}, fmt.Errorf("uploaded file must be smaller than 2MB")
	}
	img, err := decodeImageWithOrientation(raw)
	if err != nil {
		return BrandingAsset{}, fmt.Errorf("unsupported image file")
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return BrandingAsset{}, fmt.Errorf("invalid image size")
	}

	cropSize := min(bounds.Dx(), bounds.Dy())
	cropX := bounds.Min.X + (bounds.Dx()-cropSize)/2
	cropY := bounds.Min.Y + (bounds.Dy()-cropSize)/2
	cropped := image.NewNRGBA(image.Rect(0, 0, cropSize, cropSize))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: cropX, Y: cropY}, draw.Src)
	canvas := image.NewNRGBA(image.Rect(0, 0, faviconSize, faviconSize))
	scaleNearest(canvas, cropped, canvas.Bounds())
	data, err := encodePNGUnderLimitWithMax(canvas, faviconMaxBytes)
	if err != nil {
		return BrandingAsset{}, err
	}
	return BrandingAsset{
		ContentType:      "image/png",
		Data:             data,
		SizeBytes:        len(data),
		Width:            faviconSize,
		Height:           faviconSize,
		OriginalFilename: filename,
	}, nil
}

func processAvatarUpload(raw []byte) (AvatarAsset, error) {
	if len(raw) == 0 {
		return AvatarAsset{}, fmt.Errorf("uploaded file is empty")
	}
	if len(raw) > avatarMaxInputSize {
		return AvatarAsset{}, fmt.Errorf("头像图片不能超过 20MB")
	}
	img, err := decodeImageWithOrientation(raw)
	if err != nil {
		return AvatarAsset{}, fmt.Errorf("unsupported image file")
	}
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return AvatarAsset{}, fmt.Errorf("invalid image size")
	}

	cropSize := min(srcW, srcH)
	cropX := srcBounds.Min.X + (srcW-cropSize)/2
	cropY := srcBounds.Min.Y + (srcH-cropSize)/2
	cropped := image.NewNRGBA(image.Rect(0, 0, cropSize, cropSize))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: cropX, Y: cropY}, draw.Src)

	canvas := image.NewNRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	scaleNearest(canvas, cropped, canvas.Bounds())
	data, err := encodePNGUnderLimitWithMax(canvas, avatarMaxBytes)
	if err != nil {
		return AvatarAsset{}, err
	}
	return AvatarAsset{
		ContentType: "image/png",
		Data:        data,
		SizeBytes:   len(data),
		Width:       avatarSize,
		Height:      avatarSize,
	}, nil
}

func scaleNearest(dst *image.NRGBA, src image.Image, rect image.Rectangle) {
	srcBounds := src.Bounds()
	dstW := rect.Dx()
	dstH := rect.Dy()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	for y := 0; y < dstH; y++ {
		srcY := srcBounds.Min.Y + min(srcH-1, y*srcH/dstH)
		for x := 0; x < dstW; x++ {
			srcX := srcBounds.Min.X + min(srcW-1, x*srcW/dstW)
			dst.Set(rect.Min.X+x, rect.Min.Y+y, src.At(srcX, srcY))
		}
	}
}

func decodeImageWithOrientation(raw []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return applyEXIFOrientation(img, jpegEXIFOrientation(raw)), nil
}

func jpegEXIFOrientation(raw []byte) int {
	if len(raw) < 4 || raw[0] != 0xff || raw[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(raw); {
		if raw[offset] != 0xff {
			return 1
		}
		for offset < len(raw) && raw[offset] == 0xff {
			offset++
		}
		if offset >= len(raw) {
			return 1
		}
		marker := raw[offset]
		offset++
		if marker == 0xd9 || marker == 0xda {
			return 1
		}
		if offset+2 > len(raw) {
			return 1
		}
		segmentLen := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
		if segmentLen < 2 || offset+segmentLen > len(raw) {
			return 1
		}
		segment := raw[offset+2 : offset+segmentLen]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return tiffEXIFOrientation(segment[6:])
		}
		offset += segmentLen
	}
	return 1
}

func tiffEXIFOrientation(raw []byte) int {
	if len(raw) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(raw[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(raw[2:4]) != 42 {
		return 1
	}
	ifdOffset := int(order.Uint32(raw[4:8]))
	if ifdOffset < 0 || ifdOffset+2 > len(raw) {
		return 1
	}
	entryCount := int(order.Uint16(raw[ifdOffset : ifdOffset+2]))
	entryOffset := ifdOffset + 2
	for i := 0; i < entryCount; i++ {
		entry := entryOffset + i*12
		if entry+12 > len(raw) {
			return 1
		}
		tag := order.Uint16(raw[entry : entry+2])
		fieldType := order.Uint16(raw[entry+2 : entry+4])
		count := order.Uint32(raw[entry+4 : entry+8])
		if tag == 0x0112 && fieldType == 3 && count >= 1 {
			value := int(order.Uint16(raw[entry+8 : entry+10]))
			if value >= 1 && value <= 8 {
				return value
			}
			return 1
		}
	}
	return 1
}

func applyEXIFOrientation(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	dstW, dstH := w, h
	if orientation >= 5 {
		dstW, dstH = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			srcX, srcY := orientedSourcePoint(x, y, w, h, orientation)
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}

func orientedSourcePoint(x int, y int, w int, h int, orientation int) (int, int) {
	switch orientation {
	case 2:
		return w - 1 - x, y
	case 3:
		return w - 1 - x, h - 1 - y
	case 4:
		return x, h - 1 - y
	case 5:
		return y, x
	case 6:
		return y, h - 1 - x
	case 7:
		return w - 1 - y, h - 1 - x
	case 8:
		return w - 1 - y, x
	default:
		return x, y
	}
}

func encodePNGUnderLimit(img image.Image) ([]byte, error) {
	return encodePNGUnderLimitWithMax(img, logoMaxBytes)
}

func encodePNGUnderLimitWithMax(img image.Image, maxBytes int) ([]byte, error) {
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	if buf.Len() < maxBytes {
		return buf.Bytes(), nil
	}

	paletted := quantizeLogo(img)
	buf.Reset()
	if err := encoder.Encode(&buf, paletted); err != nil {
		return nil, err
	}
	if buf.Len() >= maxBytes {
		return nil, fmt.Errorf("converted png is still larger than %dKB", maxBytes/1024)
	}
	return buf.Bytes(), nil
}

func quantizeLogo(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	palette := make(color.Palette, 0, 257)
	palette = append(palette, color.NRGBA{A: 0})
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				palette = append(palette, color.NRGBA{
					R: uint8(r * 51),
					G: uint8(g * 51),
					B: uint8(b * 51),
					A: 255,
				})
			}
		}
	}
	for i := 0; i < 39; i++ {
		v := uint8(i * 255 / 38)
		palette = append(palette, color.NRGBA{R: v, G: v, B: v, A: 255})
	}

	out := image.NewPaletted(image.Rect(0, 0, bounds.Dx(), bounds.Dy()), palette)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 0x4000 {
				out.SetColorIndex(x-bounds.Min.X, y-bounds.Min.Y, 0)
				continue
			}
			idx := nearestPaletteIndex(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			out.SetColorIndex(x-bounds.Min.X, y-bounds.Min.Y, idx)
		}
	}
	return out
}

func nearestPaletteIndex(r, g, b uint8) uint8 {
	ri := int(math.Round(float64(r) / 51))
	gi := int(math.Round(float64(g) / 51))
	bi := int(math.Round(float64(b) / 51))
	ri = min(5, max(0, ri))
	gi = min(5, max(0, gi))
	bi = min(5, max(0, bi))
	return uint8(1 + ri*36 + gi*6 + bi)
}

func init() {
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("gif", "GIF8?a", gif.Decode, gif.DecodeConfig)
}
