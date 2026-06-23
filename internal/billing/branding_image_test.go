package billing

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestProcessLogoUploadKeepsCompliantPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, logoWidth, logoHeight))
	for y := 0; y < logoHeight; y++ {
		for x := 0; x < logoWidth; x++ {
			img.Set(x, y, color.NRGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	asset, err := processLogoUpload(buf.Bytes(), "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(asset.Data, buf.Bytes()) {
		t.Fatal("expected compliant png to be stored without processing")
	}
	if asset.Width != logoWidth || asset.Height != logoHeight || asset.SizeBytes != buf.Len() {
		t.Fatalf("unexpected asset metadata: %+v", asset)
	}
}

func TestProcessLogoUploadConvertsToTargetPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y * 3), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	asset, err := processLogoUpload(buf.Bytes(), "wide.png")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(asset.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != logoWidth || cfg.Height != logoHeight {
		t.Fatalf("expected %dx%d png, got %dx%d", logoWidth, logoHeight, cfg.Width, cfg.Height)
	}
	if asset.SizeBytes >= logoMaxBytes {
		t.Fatalf("expected converted logo smaller than %d bytes, got %d", logoMaxBytes, asset.SizeBytes)
	}
}

func TestProcessFaviconUploadCropsAndConvertsToSquarePNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 320, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	asset, err := processFaviconUpload(buf.Bytes(), "favicon.png")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(asset.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != faviconSize || cfg.Height != faviconSize {
		t.Fatalf("expected %dx%d png, got %dx%d", faviconSize, faviconSize, cfg.Width, cfg.Height)
	}
	if asset.ContentType != "image/png" || asset.SizeBytes >= faviconMaxBytes {
		t.Fatalf("unexpected favicon metadata: %+v", asset)
	}
}
