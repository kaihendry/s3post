package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPasswordPrompt(t *testing.T) {
	req := httptest.NewRequest("GET", "/password", nil)
	w := httptest.NewRecorder()
	passwordprompt(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSubmit(t *testing.T) {
	req := httptest.NewRequest("POST", "/setpassword", strings.NewReader("password=kensentme"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	submit(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "password" || cookies[0].Value != "kensentme" {
		t.Error("password cookie not set correctly")
	}
}

func TestHandleIndex_NoCookie(t *testing.T) {
	os.Setenv("PASSWORD", "kensentme")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect to /password, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/password" {
		t.Errorf("expected redirect to /password, got %s", loc)
	}
}

func TestHandleIndex_WrongPassword(t *testing.T) {
	os.Setenv("PASSWORD", "kensentme")
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "password", Value: "wrongpassword"})
	w := httptest.NewRecorder()
	handleIndex(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
}

func TestHandleIndex_CorrectPassword(t *testing.T) {
	os.Setenv("PASSWORD", "kensentme")
	os.Setenv("BUCKET", "s.natalian.org")
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "password", Value: "kensentme"})
	w := httptest.NewRecorder()
	handleIndex(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "s.natalian.org") {
		t.Error("expected bucket name in response")
	}
}

func TestHandleNotify_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/notify", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	handleNotify(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid JSON, got %d", w.Code)
	}
}

func TestHandleNotify_NoTopic(t *testing.T) {
	os.Unsetenv("NOTIFY_TOPIC")
	body := `{"Key":"2026-03-27/pole.jpeg","URL":"https://s.natalian.org/2026-03-27/pole.jpeg","Bucket":"s.natalian.org","ContentType":"image/jpeg"}`
	req := httptest.NewRequest("POST", "/notify", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleNotify(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when NOTIFY_TOPIC unset, got %d", w.Code)
	}
}
