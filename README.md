# surrealstore

A session store backend for [gorilla/sessions](https://github.com/gorilla/sessions) using [SurrealDB](https://surrealdb.com/).

```sh
go get -u github.com/thecomputerm/surrealstore
```

## Example

```go
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
```

To integrate it with [Gin](https://github.com/gin-gonic/gin), create a store mimicking the [postgres driver](https://github.com/gin-contrib/sessions/blob/master/postgres/postgres.go).

## Thanks

I've primarily just snatched and modified the code from [pgstore](https://github.com/antonlindstrom/pgstore) because it was the one [gin-sessions](https://github.com/gin-contrib/sessions) was using.

## Notes

- Wrapping the time.Time object in a models.CustomDateTime struct because of [this issue](https://github.com/surrealdb/surrealdb.go/issues/181).