package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/apex/gateway/v2"
	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gorilla/mux"
)

type S3upload struct {
	Key         string `json:"Key"`
	URL         string `json:"URL"`
	Bucket      string `json:"Bucket"`
	ContentType string `json:"ContentType"`
}

//go:embed static
var staticFiles embed.FS

var views = template.Must(template.ParseFS(staticFiles, "static/*.tmpl"))

func main() {
	if os.Getenv("PASSWORD") == "" {
		log.Fatal("PASSWORD environment variable must be defined")
		os.Exit(1)
	}

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.WithError(err).Fatal("failed to create static sub FS")
	}

	app := mux.NewRouter()
	app.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	app.HandleFunc("/", handleIndex).Methods("GET")
	app.HandleFunc("/notify", handleNotify).Methods("POST")
	app.HandleFunc("/setpassword", submit).Methods("POST")
	app.HandleFunc("/password", passwordprompt).Methods("GET")

	if _, ok := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME"); ok {
		err = gateway.ListenAndServe("", app)
	} else {
		err = http.ListenAndServe(":"+os.Getenv("PORT"), app)
	}
	if err != nil {
		log.WithError(err).Fatal("error listening")
	}
}

func submit(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")

	log.WithFields(log.Fields{
		"password": password,
	}).Info("submit")

	http.SetCookie(w, &http.Cookie{
		Name:    "password",
		Value:   password,
		Expires: time.Now().Add(8760 * time.Hour),
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func passwordprompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.ExecuteTemplate(w, "passwordprompt.tmpl", map[string]interface{}{})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {

	c, err := r.Cookie("password")

	if err != nil {
		log.Error("Missing cookie")
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}

	if c.Value != os.Getenv("PASSWORD") {
		log.WithFields(log.Fields{
			"password": c.Value,
		}).Error("credentials incorrect")
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}

	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-HTTPPOSTConstructPolicy.html

	// Expire in five minutes
	expInFiveMinutes := time.Now().UTC().Add(time.Minute * 5)

	// TODO model the policy in a Golang struct
	policy := fmt.Sprintf(`{"expiration": "%s",
	"conditions": [
	{ "acl": "public-read" },
	["starts-with", "$key", "%s/"],
	["starts-with", "$Content-Type", ""],
	{ "bucket": "%s" }
	]
	}`, expInFiveMinutes.Format("2006-01-02T15:04:05Z"),
		expInFiveMinutes.Format("2006-01-02"),
		os.Getenv("BUCKET"))

	b64policy := base64.StdEncoding.EncodeToString([]byte(policy))

	mac := hmac.New(sha1.New, []byte(os.Getenv("UPLOAD_SECRET")))
	mac.Write([]byte(b64policy))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.ExecuteTemplate(w, "index.tmpl", map[string]interface{}{
		"Stage":     os.Getenv("UP_STAGE"),
		"UPLOAD_ID": os.Getenv("UPLOAD_ID"),
		"BUCKET":    os.Getenv("BUCKET"),
		"REGION":    os.Getenv("REGION"),
		"Policy":    policy,
		"B64Policy": b64policy,
		"Signature": signature,
	})
}

func handleNotify(w http.ResponseWriter, r *http.Request) {

	var upload S3upload

	err := json.NewDecoder(r.Body).Decode(&upload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := log.WithFields(log.Fields{
		"reqid": r.Header.Get("X-Request-Id"),
		"UA":    r.Header.Get("User-Agent"),
		"Key":   upload.Key,
	})
	ctx.Info("Parsed payload")

	topic := os.Getenv("NOTIFY_TOPIC")
	if topic == "" {
		log.Warn("NOTIFY_TOPIC environment not setup")
		http.Error(w, "Please tell the Administrator that the notification topic is not setup", http.StatusInternalServerError)
		return
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadJSON, err := json.MarshalIndent(upload, "", "   ")
	if err != nil {
		ctx.WithError(err).Fatal("unable to marshall JSON")
	}

	client := sns.New(cfg)
	req := client.PublishRequest(&sns.PublishInput{
		TopicArn: aws.String(topic),
		Message:  aws.String(string(uploadJSON)),
	})

	ctx.Info("Attempting to send SNS")

	resp, err := req.Send(context.Background())
	if err != nil {
		log.WithError(err).Warn("unable to send SNS")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.Info("Sent")
	fmt.Fprintf(w, "%s", resp)
}
