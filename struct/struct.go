package s3post

type S3upload struct {
	Key         string `json:"Key"`
	URL         string `json:"URL"`
	Bucket      string `json:"Bucket"`
	ContentType string `json:"ContentType"`
}
