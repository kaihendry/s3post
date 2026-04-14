package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/disintegration/imaging"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	humanize "github.com/dustin/go-humanize"
	"github.com/gen2brain/avif"
)

type convert func(src string, dst string) error

type S3upload struct {
	Key         string `json:"Key"`
	URL         string `json:"URL"`
	Bucket      string `json:"Bucket"`
	ContentType string `json:"ContentType"`
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, evt events.SNSEvent) (string, error) {

	log.Infof("%+v", evt)

	uploadObject := struct {
		Key         string `json:"Key"`
		URL         string `json:"URL"`
		Bucket      string `json:"Bucket"`
		ContentType string `json:"ContentType"`
	}{}

	err := json.Unmarshal([]byte(evt.Records[0].SNS.Message), &uploadObject)
	if err != nil {
		return "", err
	}

	log.WithFields(log.Fields{
		"uploadObject": uploadObject,
	}).Info("switch")

	var info string
	var processedURL string

	switch mediatype := strings.ToLower(path.Ext(uploadObject.Key)); mediatype {
	case ".png":
		log.Info("png file")
		info, err = transcode(pngquantprocess, uploadObject, uploadObject)
		if err != nil {
			log.WithError(err).Error("failed to pngquant png file")
			return "", err
		}
		processedURL = uploadObject.URL
	case ".jpg", ".jpeg":
		log.Info("jpg file - converting to avif")
		avifObject := uploadObject
		avifObject.Key = avifObject.Key[0:len(avifObject.Key)-len(mediatype)] + ".avif"
		avifObject.URL = avifObject.URL[0:len(avifObject.URL)-len(mediatype)] + ".avif"
		avifObject.ContentType = "image/avif"
		info, err = transcode(avifprocess, uploadObject, avifObject)
		if err != nil {
			log.WithError(err).Error("failed to convert jpg to avif")
			return "", err
		}
		processedURL = avifObject.URL
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return "", err
		}
		_, err = s3.NewFromConfig(cfg).DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(uploadObject.Bucket),
			Key:    aws.String(uploadObject.Key),
		})
		if err != nil {
			log.WithError(err).Error("failed to delete source jpg")
			return "", err
		}
	case ".mov":
		log.Info("mov file")
		mp4Object := uploadObject
		mp4Object.Key = mp4Object.Key[0:len(mp4Object.Key)-len(mediatype)] + ".mp4"
		mp4Object.URL = mp4Object.URL[0:len(mp4Object.URL)-len(mediatype)] + ".mp4"
		mp4Object.ContentType = "video/mp4"
		info, err = transcode(ffmpegprocess, uploadObject, mp4Object)
		if err != nil {
			log.WithError(err).Error("failed to ffmpeg mov file")
			return "", err
		}
		processedURL = mp4Object.URL
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return "", err
		}
		_, err = s3.NewFromConfig(cfg).DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(uploadObject.Bucket),
			Key:    aws.String(uploadObject.Key),
		})
		if err != nil {
			log.WithError(err).Error("failed to delete source mov")
			return "", err
		}
	case ".heic":
		log.Info("heic file")
		jpegObject := uploadObject
		jpegObject.Key = jpegObject.Key[0:len(jpegObject.Key)-len(mediatype)] + ".jpg"
		jpegObject.URL = jpegObject.URL[0:len(jpegObject.URL)-len(mediatype)] + ".jpg"
		jpegObject.ContentType = "image/jpeg"
		info, err = transcode(heicprocess, uploadObject, jpegObject)
		if err != nil {
			log.WithError(err).Error("failed to convert heic to jpeg")
			return "", err
		}
		processedURL = jpegObject.URL
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return "", err
		}
		_, err = s3.NewFromConfig(cfg).DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(uploadObject.Bucket),
			Key:    aws.String(uploadObject.Key),
		})
		if err != nil {
			log.WithError(err).Error("failed to delete source heic")
			return "", err
		}
	case ".webp":
		log.Info("webp file")
		info, err = transcode(cwebpprocess, uploadObject, uploadObject)
		if err != nil {
			log.WithError(err).Error("failed to cwebp webp file")
			return "", err
		}
		processedURL = uploadObject.URL
	case ".mp4":
		log.Info("mp4 file - already processed, notifying")
		processedURL = uploadObject.URL
	default:
		log.Warnf("unrecognized %s", mediatype)
	}

	if processedURL != "" {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return "", err
		}
		_, err = sns.NewFromConfig(cfg).Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(os.Getenv("TOPIC")),
			Message:  aws.String(fmt.Sprintf("%s\n%s", processedURL, info)),
		})
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func put(src string, dst S3upload) (err error) {
	log.Infof("Putting %s on %v", src, dst)

	f, err := os.Open(src)
	if err != nil {
		log.WithError(err).Fatal("unable to open src")
		return err
	}
	defer f.Close()

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	_, err = s3.NewFromConfig(cfg).PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(dst.Bucket),
		Body:        f,
		Key:         aws.String(dst.Key),
		ContentType: aws.String(dst.ContentType),
	})
	if err != nil {
		log.WithError(err).Fatal("failed to upload to s3")
	}
	return err
}

func get(src S3upload, dst string) (err error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	res, err := s3.NewFromConfig(cfg).GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(src.Bucket),
		Key:    aws.String(src.Key),
	})
	if err != nil {
		log.WithError(err).Fatal("failed to get file")
		return err
	}
	defer res.Body.Close()

	outFile, err := os.Create(dst)
	if err != nil {
		log.WithError(err).Fatal("failed to create output file")
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, res.Body)
	return err
}

func transcode(fn convert, srcObject S3upload, dstObject S3upload) (info string, err error) {

	// foo to get tempfile ending in foo$
	srctmpfile, err := os.CreateTemp("", "*"+filepath.Ext(srcObject.Key))
	if err != nil {
		log.WithError(err).Fatal("failed to create temp input file")
		return "", err
	}

	src := srctmpfile.Name()
	err = get(srcObject, src)
	defer os.Remove(srctmpfile.Name())

	if err != nil {
		log.WithError(err).Error("failed to retrieve src file to lambda")
		return "", err
	}

	tmpfile, err := os.CreateTemp("", "*"+filepath.Ext(dstObject.Key))
	if err != nil {
		log.WithError(err).Error("failed to create temp output file")
		return "", err
	}

	dst := tmpfile.Name()
	defer os.Remove(tmpfile.Name())

	err = fn(src, dst)

	if err != nil {
		log.WithError(err).Error("failed to transcode")
		return "", err
	}

	err = put(dst, dstObject)
	if err != nil {
		log.WithError(err).Error("failed to put")
		return "", err
	}

	info = fmt.Sprintf("Transcode size: %s → %s", size(src), size(dst))
	if strings.ToLower(filepath.Ext(dstObject.Key)) == ".mp4" {
		if res, rerr := resolution(dst); rerr == nil {
			info = fmt.Sprintf("%s (%s)", info, res)
		}
	}
	return info, err

}

// lookPath finds a binary by checking ./name first (for Lambda bundled binaries),
// then falling back to PATH.
func lookPath(name string) (string, error) {
	if p, err := exec.LookPath("./" + name); err == nil {
		return p, nil
	}
	return exec.LookPath(name)
}

func ffmpegprocess(src string, dst string) (err error) {
	path, err := lookPath("ffmpeg")
	if err != nil {
		log.WithError(err).Error("no ffmpeg binary found")
		return err
	}
	log.Infof("Launching ffmpeg: %s -> %s", src, dst)
	cmd := exec.Command(path, "-y", "-i", src, "-movflags", "+faststart", "-c:v", "libx264", dst)
	// create a pipe for the output of the script
	// https://stackoverflow.com/a/48381051/4534
	cmdReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			log.Debugf("\t > %s\n", scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return err
}

func heicprocess(src string, dst string) (err error) {
	p, err := lookPath("heif-convert")
	if err != nil {
		log.WithError(err).Error("no heif-convert binary found")
		return err
	}
	log.Infof("Converting HEIC to JPEG: %s -> %s", src, dst)
	absP, _ := filepath.Abs(p)
	cmd := exec.Command(p, src, dst)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(absP))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Errorf("heif-convert failed: %s", out)
	}
	return err
}

func pngquantprocess(src string, dst string) (err error) {
	var out []byte
	path, err := lookPath("pngquant")
	if err != nil {
		log.WithError(err).Error("no pngquant binary found")
		return err
	}
	out, err = exec.Command(path, "-f", src, "-o", dst).CombinedOutput()
	if err != nil {
		log.WithError(err).Errorf("pngquant failed: %s", out)
		return err
	}
	return err
}

func cjpegprocess(src string, dst string) (err error) {
	var out []byte
	p, err := lookPath("jpegtran")
	if err != nil {
		log.WithError(err).Error("no jpegtran binary found")
		return err
	}
	// -copy all preserves EXIF (including Orientation) so rotation is not lost
	cmd := exec.Command(p, "-copy", "all", "-optimize", "-outfile", dst, src)
	absP, _ := filepath.Abs(p)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(absP))
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Errorf("jpegtran failed: %s", out)
		return err
	}
	return err
}

func avifprocess(src string, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	// AutoOrientation applies the EXIF Orientation tag so rotation is correct
	img, err := imaging.Decode(f, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	return avif.Encode(out, img)
}

func cwebpprocess(src string, dst string) (err error) {
	var out []byte
	path, err := lookPath("cwebp")
	if err != nil {
		log.WithError(err).Error("no cwebp binary found")
		return err
	}
	out, err = exec.Command(path, "-q", "80", src, "-o", dst).CombinedOutput()
	if err != nil {
		log.WithError(err).Errorf("cwebp failed: %s", out)
		return err
	}
	return err
}

func size(file string) string {
	stats, _ := os.Stat(file)
	return humanize.Bytes(uint64(stats.Size()))
}

func resolution(file string) (string, error) {
	path, err := lookPath("ffprobe")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		file,
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
