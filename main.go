package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"html/template"

	"github.com/apex/log"
	"github.com/gorilla/mux"
)

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	app.HandleFunc("/", handleIndex).Methods("GET")

	if err := http.ListenAndServe(addr, app); err != nil {
		log.WithError(err).Fatal("error listening")
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("UP_STAGE") != "production" {
		w.Header().Set("X-Robots-Tag", "none")
	}

	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-HTTPPOSTConstructPolicy.html

	exp := time.Now().UTC().Add(time.Second * time.Duration(60))

	policy := fmt.Sprintf(`{"expiration": "%s",
	"conditions": [
	{ "acl": "public-read" },
	["starts-with", "$key", "%s/"],
	["starts-with", "$Content-Type", ""],
	{ "bucket": "%s" }
	]
	}`, exp.Format("2006-01-02T15:04:05Z"), exp.Format("2006-01-02"), os.Getenv("BUCKET"))

	mac := hmac.New(sha1.New, []byte(os.Getenv("AWS_SECRET_ACCESS_KEY")))
	mac.Write([]byte(policy))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	t := template.Must(template.New("").ParseGlob("templates/*.tmpl"))
	t.ExecuteTemplate(w, "index.tmpl", map[string]interface{}{
		"Stage":             os.Getenv("UP_STAGE"),
		"AWS_ACCESS_KEY_ID": os.Getenv("AWS_ACCESS_KEY_ID"),
		"BUCKET":            os.Getenv("BUCKET"),
		"REGION":            os.Getenv("REGION"),
		"Policy":            policy,
		"Signature":         signature,
	})
}
