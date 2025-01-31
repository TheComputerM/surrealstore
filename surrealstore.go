package surrealstore

import (
	"encoding/base32"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type SurrealStore struct {
	Codecs  []securecookie.Codec
	Options *sessions.Options
	DB      *surrealdb.DB
}

type SurrealSession struct {
	ID         *models.RecordID      `json:"id,omitempty"`
	Data       string                `json:"data"`
	CreatedOn  models.CustomDateTime `json:"created_on"`
	ModifiedOn models.CustomDateTime `json:"modified_on"`
	ExpiresOn  models.CustomDateTime `json:"expires_on"`
}

func NewSurrealStore(db *surrealdb.DB, keyPairs ...[]byte) (*SurrealStore, error) {
	store := &SurrealStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   "/",
			MaxAge: 86400 * 30,
		},
		DB: db,
	}

	if err := store.createSessionsTable(); err != nil {
		return nil, err
	}

	return store, nil
}

func (store *SurrealStore) Close() {
	store.DB.Close()
}

func (store *SurrealStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(store, name)
}

// New returns a new session for the given name without adding it to the registry.
func (store *SurrealStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(store, name)
	if session == nil {
		return nil, nil
	}

	opts := *store.Options
	session.Options = &(opts)
	session.IsNew = true

	var err error
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, store.Codecs...)
		if err == nil {
			err = store.load(session)
			if err == nil {
				session.IsNew = false
			} else {
				err = nil
			}
		}
	}

	store.MaxAge(store.Options.MaxAge)

	return session, err
}

// Save saves the given session into the database and deletes cookies if needed
func (store *SurrealStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Set delete if max-age is < 0
	if session.Options.MaxAge < 0 {
		if err := store.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		// Generate a random session ID key suitable for storage in the DB
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	}

	if err := store.save(session); err != nil {
		return err
	}

	// Keep the session ID key in a cookie so it can be looked up in DB later.
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, store.Codecs...)
	if err != nil {
		return err
	}

	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default for a new PGStore is 4096.
func (db *SurrealStore) MaxLength(l int) {
	for _, c := range db.Codecs {
		if codec, ok := c.(*securecookie.SecureCookie); ok {
			codec.MaxLength(l)
		}
	}
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting Options.MaxAge
// = -1 for that session.
func (db *SurrealStore) MaxAge(age int) {
	db.Options.MaxAge = age

	// Set the maxAge for each securecookie instance.
	for _, codec := range db.Codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

// load fetches a session by ID from the database and decodes its content
// into session.Values.
func (store *SurrealStore) load(session *sessions.Session) error {
	entry, err := surrealdb.Select[SurrealSession](store.DB, models.NewRecordID("sessions", session.ID))
	if err != nil {
		return err
	}
	return securecookie.DecodeMulti(session.Name(), entry.Data, &session.Values, store.Codecs...)
}

func (store *SurrealStore) save(session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, store.Codecs...)
	if err != nil {
		return err
	}

	crOn := session.Values["created_on"]
	exOn := session.Values["expires_on"]

	var expiresOn time.Time

	createdOn, ok := crOn.(time.Time)
	if !ok {
		createdOn = time.Now()
	}

	if exOn == nil {
		expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
	} else {
		expiresOn = exOn.(time.Time)
		if expiresOn.Sub(time.Now().Add(time.Second*time.Duration(session.Options.MaxAge))) < 0 {
			expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
		}
	}

	s := SurrealSession{
		ID:         &models.RecordID{Table: "sessions", ID: session.ID},
		Data:       encoded,
		CreatedOn:  models.CustomDateTime{Time: createdOn},
		ExpiresOn:  models.CustomDateTime{Time: expiresOn},
		ModifiedOn: models.CustomDateTime{Time: time.Now()},
	}

	_, err = surrealdb.Upsert[SurrealSession](store.DB, *s.ID, s)

	return err
}

// Delete session
func (store *SurrealStore) delete(session *sessions.Session) error {
	_, err := surrealdb.Delete[models.RecordID](store.DB, models.NewRecordID("sessions", session.ID))
	return err
}

func (store *SurrealStore) createSessionsTable() error {
	query := `DEFINE TABLE IF NOT EXISTS sessions SCHEMALESS`

	_, err := surrealdb.Query[any](store.DB, query, map[string]interface{}{})

	return err
}
