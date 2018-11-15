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

var views = template.Must(template.ParseGlob("templates/*.tmpl"))

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	app.HandleFunc("/", handleIndex).Methods("GET")
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
		Expires: time.Now().Add(8760 * time.Hour),
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

	exp := time.Now().UTC().Add(time.Second * time.Duration(60))

	policy := fmt.Sprintf(`{"expiration": "%s",
	"conditions": [
	{ "acl": "public-read" },
	["starts-with", "$key", "%s/"],
	["starts-with", "$Content-Type", ""],
	{ "bucket": "%s" }
	]
	}`, exp.Format("2006-01-02T15:04:05Z"), exp.Format("2006-01-02"), os.Getenv("BUCKET"))

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
