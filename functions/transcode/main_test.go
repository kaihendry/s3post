package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestHeicprocess(t *testing.T) {
	if _, err := lookPath("heif-convert"); err != nil {
		t.Skip("heif-convert not found in PATH")
	}
	src := "/Users/hendry/Downloads/IMG_2618.HEIC"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("test fixture not found: %s", src)
	}

	dst := filepath.Join(t.TempDir(), "out.jpg")
	if err := heicprocess(src, dst); err != nil {
		t.Fatalf("heicprocess failed: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
	// Verify it's a valid JPEG by checking magic bytes
	f, _ := os.Open(dst)
	defer f.Close()
	magic := make([]byte, 3)
	f.Read(magic)
	if magic[0] != 0xFF || magic[1] != 0xD8 {
		t.Fatalf("output is not a valid JPEG (magic: %x)", magic)
	}
	t.Logf("HEIC->JPEG: %d bytes", info.Size())
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

func TestCjpegprocessPreservesOrientation(t *testing.T) {
	if _, err := lookPath("jpegtran"); err != nil {
		t.Skip("jpegtran not found in PATH")
	}
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not found in PATH")
	}

	src := writeJPEG(t)
	defer os.Remove(src)

	// Embed Orientation=6 (90° CW rotation) into the JPEG
	out, err := exec.Command("exiftool", "-overwrite_original", "-Orientation=6", "-n", src).CombinedOutput()
	if err != nil {
		t.Fatalf("exiftool set orientation failed: %v: %s", err, out)
	}

	dst := filepath.Join(t.TempDir(), "out.jpg")
	if err := cjpegprocess(src, dst); err != nil {
		t.Fatalf("cjpegprocess failed: %v", err)
	}

	out, err = exec.Command("exiftool", "-Orientation", "-n", dst).CombinedOutput()
	if err != nil {
		t.Fatalf("exiftool read failed: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "6") {
		t.Fatalf("EXIF Orientation not preserved, got: %s", out)
	}
}

func TestAvifprocess(t *testing.T) {
	src := writeJPEG(t)
	defer os.Remove(src)

	dst := filepath.Join(t.TempDir(), "out.avif")
	if err := avifprocess(src, dst); err != nil {
		t.Fatalf("avifprocess failed: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output AVIF is empty")
	}
	// AVIF files start with ftyp box; check for "avif" brand at offset 8
	f, _ := os.Open(dst)
	defer f.Close()
	buf := make([]byte, 12)
	f.Read(buf)
	if string(buf[8:12]) != "avif" && string(buf[8:12]) != "avis" {
		t.Fatalf("output does not look like AVIF (bytes 8-12: %q)", buf[8:12])
	}
	t.Logf("JPEG->AVIF: %d bytes", info.Size())
}
