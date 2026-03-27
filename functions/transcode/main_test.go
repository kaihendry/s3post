package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "*.png")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	out, err := os.Create(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func writeJPEG(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	out, err := os.Create(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := jpeg.Encode(out, img, nil); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestPngquantprocess(t *testing.T) {
	if _, err := lookPath("pngquant"); err != nil {
		t.Skip("pngquant not found in PATH")
	}
	src := writePNG(t)
	defer os.Remove(src)

	dst := filepath.Join(t.TempDir(), "out.png")
	if err := pngquantprocess(src, dst); err != nil {
		t.Fatalf("pngquantprocess failed: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

func TestCjpegprocess(t *testing.T) {
	if _, err := lookPath("jpegtran"); err != nil {
		t.Skip("jpegtran not found in PATH")
	}
	src := writeJPEG(t)
	defer os.Remove(src)

	dst := filepath.Join(t.TempDir(), "out.jpg")
	if err := cjpegprocess(src, dst); err != nil {
		t.Fatalf("cjpegprocess failed: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

func TestCwebpprocess(t *testing.T) {
	if _, err := lookPath("cwebp"); err != nil {
		t.Skip("cwebp not found in PATH")
	}
	src := writeJPEG(t)
	defer os.Remove(src)

	dst := filepath.Join(t.TempDir(), "out.webp")
	if err := cwebpprocess(src, dst); err != nil {
		t.Fatalf("cwebpprocess failed: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

func TestFfmpegprocess(t *testing.T) {
	p, err := lookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found in PATH")
	}
	// Create a minimal 1-second silent video as test fixture
	src := filepath.Join(t.TempDir(), "in.mov")
	out, err := exec.Command(p, "-f", "lavfi", "-i", "color=c=black:size=64x64:duration=1:rate=1", "-f", "mov", src).CombinedOutput()
	if err != nil {
		t.Skipf("could not create test mov: %v\n%s", err, out)
	}

	dst := filepath.Join(t.TempDir(), "out.mp4")
	if err := ffmpegprocess(src, dst); err != nil {
		t.Fatalf("ffmpegprocess failed: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}
