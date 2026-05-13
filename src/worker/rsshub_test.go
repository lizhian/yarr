package worker

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRSSHubBaseAcceptsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/rsshub", http.StatusFound)
	}))
	defer server.Close()

	if got := checkRSSHubBase(server.URL); got != rsshubAvailable {
		t.Fatalf("got %v", got)
	}
}

func TestCheckRSSHubBaseRejectsErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if got := checkRSSHubBase(server.URL); got != rsshubUnavailable {
		t.Fatalf("got %v", got)
	}
}
