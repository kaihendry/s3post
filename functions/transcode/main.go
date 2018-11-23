package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/apex/log"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3post "github.com/kaihendry/s3post/struct"
)

type S3PostSNS struct {
	Records []struct {
		EventSource          string `json:"EventSource"`
		EventVersion         string `json:"EventVersion"`
		EventSubscriptionArn string `json:"EventSubscriptionArn"`
		Sns                  struct {
			Type              string      `json:"Type"`
			MessageID         string      `json:"MessageId"`
			TopicArn          string      `json:"TopicArn"`
			Subject           interface{} `json:"Subject"`
			Message           string      `json:"Message"`
			Timestamp         time.Time   `json:"Timestamp"`
			SignatureVersion  string      `json:"SignatureVersion"`
			Signature         string      `json:"Signature"`
			SigningCertURL    string      `json:"SigningCertUrl"`
			UnsubscribeURL    string      `json:"UnsubscribeUrl"`
			MessageAttributes struct {
			} `json:"MessageAttributes"`
		} `json:"Sns"`
	} `json:"Records"`
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, evt S3PostSNS) (string, error) {

	var uploadObject s3post.S3upload

	err := json.Unmarshal([]byte(evt.Records[0].Sns.Message), &uploadObject)
	if err != nil {
		return "", err
	}

	suffix := filepath.Ext(uploadObject.Key)
	log.WithFields(log.Fields{
		"suffix":       suffix,
		"uploadObject": uploadObject,
	}).Info("switch")

	switch suffix {
	case ".txt":
		log.Info("txt file")
		var out []byte
		path, err := exec.LookPath("./hello/hello")
		if err != nil {
			log.WithError(err).Error("no hello binary found")
			return "", err
		}
		out, err = exec.Command(path).CombinedOutput()
		if err != nil {
			log.WithError(err).Errorf("hello failed: %s", out)
			return string(out), err
		}

		cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("mine"))
		if err != nil {
			return "", err
		}

		svc := s3.New(cfg)

		putparams := &s3.PutObjectInput{
			Bucket:      aws.String(uploadObject.Bucket),
			Body:        bytes.NewReader(out),
			Key:         aws.String(uploadObject.Key),
			ACL:         s3.ObjectCannedACLPublicRead,
			ContentType: aws.String("text/plain; charset=UTF-8"),
		}

		req := svc.PutObjectRequest(putparams)
		_, err = req.Send()
		if err != nil {
			log.WithError(err).Fatal("failed to upload to s3")
			return "", err
		}

	default:
		log.Warn("unrecognized suffix")
	}

	return "", nil
}
