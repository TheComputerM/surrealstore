package surrealstore_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/thecomputerm/surrealstore"
)

func TestCleanup(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000")
	if err != nil {
		t.Fatal("This test requires a real database.")
	}

	if _, err = db.SignIn(&surrealdb.Auth{
		Username: "root",
		Password: "root",
	}); err != nil {
		t.Error(err.Error())
	}

	if err = db.Use("default", "auth"); err != nil {
		t.Error(err.Error())
	}

	ss, err := surrealstore.NewSurrealStore(db, []byte(secret))
	if err != nil {
		t.Fatal("Failed to get store", err)
	}

	defer ss.DB.Close()

	// Start the cleanup goroutine.
	defer ss.StopCleanup(ss.Cleanup(time.Millisecond * 500))

	req, err := http.NewRequest("GET", "http://www.example.com", nil)
	if err != nil {
		t.Fatal("Failed to create request", err)
	}

	session, err := ss.Get(req, "newsess")
	if err != nil {
		t.Fatal("Failed to create session", err)
	}

	// Expire the session.
	session.Options.MaxAge = 1

	m := make(http.Header)
	if err = ss.Save(req, headerOnlyResponseWriter(m), session); err != nil {
		t.Fatal("failed to save session:", err.Error())
	}

	// Give the ticker a moment to run.
	time.Sleep(time.Millisecond * 1500)

	// SELECT expired sessions. We should get a count of zero back.
	data, err := surrealdb.Query[int](
		ss.DB,
		"count(SELECT id FROM sessions WHERE expires_on < time::now());",
		map[string]interface{}{},
	)
	count := (*data)[0].Result
	if err != nil {
		t.Fatalf("failed to select expired sessions from DB: %v", err)
	}

	if count > 0 {
		t.Fatalf("ticker did not delete expired sessions: want 0 got %v", count)
	}
}
