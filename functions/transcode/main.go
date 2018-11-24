package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	humanize "github.com/dustin/go-humanize"
	"github.com/kaihendry/aws-lambda-go/events"
	s3post "github.com/kaihendry/s3post/struct"
)

type convert func(src string, dst string) error

var cfg aws.Config

func main() {
	cfg, _ = external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("mine"))
	lambda.Start(handler)
}

func handler(ctx context.Context, evt events.SNSEvent) (string, error) {

	var uploadObject s3post.S3upload

	err := json.Unmarshal([]byte(evt.Records[0].SNS.Message), &uploadObject)
	if err != nil {
		return "", err
	}

	log.WithFields(log.Fields{
		"uploadObject": uploadObject,
	}).Info("switch")

	var processedURL string

	switch mediatype := strings.ToLower(path.Ext(uploadObject.Key)); mediatype {
	case ".png":
		log.Info("png file")
		err = transcode(pngquantprocess, uploadObject, uploadObject)
		if err != nil {
			log.WithError(err).Error("failed to pngquant png file")
			return "", err
		}
	case ".mov":
		log.Info("mov file")
		mp4Object := uploadObject
		mp4Object.Key = mp4Object.Key[0:len(mp4Object.Key)-len(mediatype)] + ".mp4"
		mp4Object.URL = mp4Object.URL[0:len(mp4Object.URL)-len(mediatype)] + ".mp4"
		mp4Object.ContentType = "video/mp4"
		err = transcode(ffmpegprocess, uploadObject, mp4Object)
		if err != nil {
			log.WithError(err).Error("failed to ffmpeg mov file")
			return "", err
		}
		processedURL = mp4Object.URL
	default:
		log.Warnf("unrecognized %s", mediatype)
	}

	if processedURL != "" {
		client := sns.New(cfg)
		req := client.PublishRequest(&sns.PublishInput{
			TopicArn: aws.String(os.Getenv("TOPIC")),
			Message:  aws.String(processedURL),
		})
		_, err := req.Send()
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func put(src string, dst s3post.S3upload) (err error) {
	log.Infof("Putting %s on %v", src, dst)
	svc := s3.New(cfg)

	f, err := os.Open(src)
	if err != nil {
		log.WithError(err).Fatal("unable to open src")
		return err
	}
	defer f.Close()

	putparams := &s3.PutObjectInput{
		Bucket:      aws.String(dst.Bucket),
		Body:        aws.ReadSeekCloser(f),
		Key:         aws.String(dst.Key),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String(fmt.Sprintf("%s; charset=UTF-8", dst.ContentType)),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()
	if err != nil {
		log.WithError(err).Fatal("failed to upload to s3")
		return err
	}

	return nil
}

func get(src s3post.S3upload, dst string) (err error) {

	svc := s3.New(cfg)

	input := &s3.GetObjectInput{
		Bucket: aws.String(src.Bucket),
		Key:    aws.String(src.Key),
	}

	req := svc.GetObjectRequest(input)
	res, err := req.Send()
	if err != nil {
		log.WithError(err).Fatal("failed to get file")
		return err
	}

	outFile, err := os.Create(dst)
	if err != nil {
		log.WithError(err).Fatal("failed to create output file")
		return err
	}

	defer outFile.Close()
	_, err = io.Copy(outFile, res.Body)

	return err
}

func transcode(fn convert, srcObject s3post.S3upload, dstObject s3post.S3upload) (err error) {

	// foo to get tempfile ending in foo$
	srctmpfile, err := ioutil.TempFile("", "*"+filepath.Ext(srcObject.Key))
	if err != nil {
		log.WithError(err).Fatal("failed to create temp input file")
		return err
	}

	src := srctmpfile.Name()
	err = get(srcObject, src)
	defer os.Remove(srctmpfile.Name())

	if err != nil {
		log.WithError(err).Error("failed to retrieve src file to lambda")
		return err
	}

	tmpfile, err := ioutil.TempFile("", "*"+filepath.Ext(dstObject.Key))
	if err != nil {
		log.WithError(err).Error("failed to create temp output file")
		return err
	}

	dst := tmpfile.Name()
	defer os.Remove(tmpfile.Name())

	err = fn(src, dst)
	log.Infof("Transcode size: %s -> %s", size(src), size(dst))

	if err != nil {
		log.WithError(err).Error("failed to transcode")
		return err
	}

	err = put(dst, dstObject)
	if err != nil {
		log.WithError(err).Error("failed to put")
		return err
	}

	return err

}

func ffmpegprocess(src string, dst string) (err error) {
	path, err := exec.LookPath("./ffmpeg/ffmpeg")
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
	path, err := exec.LookPath("./pngquant/pngquant")
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

func size(file string) string {
	stats, _ := os.Stat(file)
	return humanize.Bytes(uint64(stats.Size()))
}
