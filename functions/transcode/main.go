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
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	humanize "github.com/dustin/go-humanize"
	"github.com/aws/aws-lambda-go/events"
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
		log.Info("jpg file")
		info, err = transcode(cjpegprocess, uploadObject, uploadObject)
		if err != nil {
			log.WithError(err).Error("failed to jpegtran jpg file")
			return "", err
		}
		processedURL = uploadObject.URL
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
	case ".webp":
		log.Info("webp file")
		info, err = transcode(cwebpprocess, uploadObject, uploadObject)
		if err != nil {
			log.WithError(err).Error("failed to cwebp webp file")
			return "", err
		}
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

	return fmt.Sprintf("Transcode size: %s → %s", size(src), size(dst)), err

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
	cmd := exec.Command(p, "-optimize", "-outfile", dst, src)
	// Bundle libjpeg.so.62 alongside the binary; point the linker to its directory.
	absP, _ := filepath.Abs(p)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(absP))
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Errorf("jpegtran failed: %s", out)
		return err
	}
	return err
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
