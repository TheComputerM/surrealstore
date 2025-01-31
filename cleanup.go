package surrealstore

import (
	"log"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

var defaultInterval = time.Minute * 5

// Cleanup runs a background goroutine every interval that deletes expired
// sessions from the database.
//
// The design is based on https://github.com/yosssi/boltstore
func (store *SurrealStore) Cleanup(interval time.Duration) (chan<- struct{}, <-chan struct{}) {
	if interval <= 0 {
		interval = defaultInterval
	}

	quit, done := make(chan struct{}), make(chan struct{})
	go store.cleanup(interval, quit, done)
	return quit, done
}

// StopCleanup stops the background cleanup from running.
func (store *SurrealStore) StopCleanup(quit chan<- struct{}, done <-chan struct{}) {
	quit <- struct{}{}
	<-done
}

// cleanup deletes expired sessions at set intervals.
func (store *SurrealStore) cleanup(interval time.Duration, quit <-chan struct{}, done chan<- struct{}) {
	ticker := time.NewTicker(interval)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-quit:
			// Handle the quit signal.
			done <- struct{}{}
			return
		case <-ticker.C:
			// Delete expired sessions on each tick.
			if err := store.deleteExpired(); err != nil {
				log.Printf("surrealstore: unable to delete expired sessions: %v", err)
			}
		}
	}
}

// deleteExpired deletes expired sessions from the database.
func (store *SurrealStore) deleteExpired() error {
	_, err := surrealdb.Query[any](store.DB, "DELETE sessions WHERE expires_on < time::now()", map[string]interface{}{})
	return err
}
