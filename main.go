package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apex/gateway/v2"
	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	app.HandleFunc("/presign", handlePresign).Methods("GET")
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

func authenticated(r *http.Request) bool {
	c, err := r.Cookie("password")
	return err == nil && c.Value == os.Getenv("PASSWORD")
}

func submit(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	log.WithFields(log.Fields{"password": password}).Info("submit")
	http.SetCookie(w, &http.Cookie{
		Name:    "password",
		Value:   password,
		Expires: time.Now().Add(8760 * time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func passwordprompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.ExecuteTemplate(w, "passwordprompt.tmpl", nil)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r) {
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.ExecuteTemplate(w, "index.tmpl", map[string]string{
		"BUCKET": os.Getenv("BUCKET"),
	})
}

// handlePresign returns a presigned PUT URL for a given S3 key.
// The key must start with today's date prefix (YYYY-MM-DD/).
func handlePresign(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	key := r.URL.Query().Get("key")
	today := time.Now().UTC().Format("2006-01-02")
	if key == "" || !strings.HasPrefix(key, today+"/") {
		http.Error(w, "key must start with today's date prefix ("+today+"/)", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadDefaultConfig(r.Context())
	if err != nil {
		log.WithError(err).Error("loading AWS config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	presigner := s3.NewPresignClient(s3.NewFromConfig(cfg))
	req, err := presigner.PresignPutObject(r.Context(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("BUCKET")),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		log.WithError(err).Error("presigning PUT")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": req.URL})
}

func handleNotify(w http.ResponseWriter, r *http.Request) {
	var upload S3upload
	if err := json.NewDecoder(r.Body).Decode(&upload); err != nil {
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
		http.Error(w, "notification topic is not configured", http.StatusInternalServerError)
		return
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.WithError(err).Error("loading AWS config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadJSON, err := json.MarshalIndent(upload, "", "   ")
	if err != nil {
		ctx.WithError(err).Fatal("unable to marshal JSON")
	}

	ctx.Info("Attempting to send SNS")
	result, err := sns.NewFromConfig(cfg).Publish(context.Background(), &sns.PublishInput{
		TopicArn: aws.String(topic),
		Message:  aws.String(string(uploadJSON)),
	})
	if err != nil {
		log.WithError(err).Warn("unable to send SNS")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.Info("Sent")
	fmt.Fprintf(w, "%s", aws.ToString(result.MessageId))
}
