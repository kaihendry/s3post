package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"html/template"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gorilla/mux"
	s3post "github.com/kaihendry/s3post/struct"
)

var views = template.Must(template.ParseGlob("static/*.tmpl"))

func init() {
	if os.Getenv("PASSWORD") == "" {
		log.Fatal("PASSWORD environment variable must be defined")
		os.Exit(1)
	}
}

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	app.HandleFunc("/", handleIndex).Methods("GET")
	app.HandleFunc("/notify", handleNotify).Methods("POST")
	app.HandleFunc("/setpassword", submit).Methods("POST")
	app.HandleFunc("/password", passwordprompt).Methods("GET")

	if err := http.ListenAndServe(addr, app); err != nil {
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
		Expires: time.Now().Add(8760 * time.Hour), // Expire in a year?
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func passwordprompt(w http.ResponseWriter, r *http.Request) {
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
	}`, expInFiveMinutes.Format("2006-01-02T15:04:05Z"), expInFiveMinutes.Format("2006-01-02"), os.Getenv("BUCKET"))

	b64policy := base64.StdEncoding.EncodeToString([]byte(policy))

	mac := hmac.New(sha1.New, []byte(os.Getenv("UPLOAD_SECRET")))
	mac.Write([]byte(b64policy))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

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

	// TODO Check cookie?

	var upload s3post.S3upload

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
		http.Error(w, fmt.Sprintf("Please tell the Administrator that the notification topic is not setup"), http.StatusInternalServerError)
		return
	}

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("mine"))
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID

	uploadJSON, _ := json.MarshalIndent(upload, "", "   ")

	client := sns.New(cfg)
	req := client.PublishRequest(&sns.PublishInput{
		TopicArn: aws.String(topic),
		Message:  aws.String(string(uploadJSON)),
	})
	resp, err := req.Send()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.Info("Sent")
	fmt.Fprintf(w, "%s", resp)
}
