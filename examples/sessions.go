package examples

import (
	"log"
	"net/http"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/thecomputerm/surrealstore"
)

// ExampleHandler is an example that displays the usage of PGStore.
func ExampleHandler(w http.ResponseWriter, r *http.Request) {
	db, err := surrealdb.New("ws://localhost:8000")
	if err != nil {
		log.Fatalln("Requires a real instance of surrealdb listening on localhost:8000.")
	}

	if _, err = db.SignIn(&surrealdb.Auth{
		Username: "root",
		Password: "root",
	}); err != nil {
		log.Fatalf("Failed to sign in to surrealdb: %v", err)
	}

	if err = db.Use("default", "auth"); err != nil {
		log.Fatalf("Failed to use correct namespace and db: %v", err)
	}

	// Fetch new store.
	store, err := surrealstore.NewSurrealStore(db, []byte("secret-key"))
	if err != nil {
		log.Fatalf("Failed to create a store: %v", err)
	}
	defer store.Close()

	// Run a background goroutine to clean up expired sessions from the database.
	defer store.StopCleanup(store.Cleanup(time.Minute * 5))

	// Get a session.
	session, err := store.Get(r, "session-key")
	if err != nil {
		log.Fatalf("Error getting session: %v", err)
	}

	// Add a value.
	session.Values["foo"] = "bar"

	// Save.
	if err = session.Save(r, w); err != nil {
		log.Fatalf("Error saving session: %v", err)
	}

	// Delete session.
	session.Options.MaxAge = -1
	if err = session.Save(r, w); err != nil {
		log.Fatalf("Error saving session: %v", err)
	}
}
