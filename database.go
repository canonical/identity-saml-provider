package main

import (
	"database/sql"
	"log"

	"github.com/crewjam/saml"
	"github.com/lib/pq"
)

// initDatabase creates the sessions and service_providers tables if they don't exist
func initDatabase() error {
	query := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			create_time TIMESTAMPTZ NOT NULL,
			expire_time TIMESTAMPTZ NOT NULL,
			index_val TEXT NOT NULL,
			name_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			user_common_name TEXT NOT NULL,
			groups TEXT[] DEFAULT '{}'
		);
		
		CREATE INDEX IF NOT EXISTS idx_sessions_expire_time ON sessions(expire_time);
		
		CREATE TABLE IF NOT EXISTS service_providers (
			entity_id TEXT PRIMARY KEY,
			acs_url TEXT NOT NULL,
			acs_binding TEXT NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}
	log.Println("Database schema initialized")
	return nil
}

// saveSessionToDB saves a SAML session to the database
func saveSessionToDB(session *saml.Session) error {
	log.Printf("Saving session to database: ID=%s, Email=%s, ExpireTime=%s", session.ID, session.UserEmail, session.ExpireTime)
	query := `
		INSERT INTO sessions (id, create_time, expire_time, index_val, name_id, user_email, user_common_name, groups)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			create_time = EXCLUDED.create_time,
			expire_time = EXCLUDED.expire_time,
			index_val = EXCLUDED.index_val,
			name_id = EXCLUDED.name_id,
			user_email = EXCLUDED.user_email,
			user_common_name = EXCLUDED.user_common_name,
			groups = EXCLUDED.groups
	`
	_, err := db.Exec(query,
		session.ID,
		session.CreateTime,
		session.ExpireTime,
		session.Index,
		session.NameID,
		session.UserEmail,
		session.UserCommonName,
		pq.Array(session.Groups),
	)
	if err != nil {
		log.Printf("Error saving session to database: %v", err)
	} else {
		log.Printf("Session saved successfully to database: ID=%s", session.ID)
	}
	return err
}

// getSessionFromDB retrieves a SAML session from the database by ID
func getSessionFromDB(sessionID string) *saml.Session {
	log.Printf("Attempting to retrieve session from database: ID=%s", sessionID)

	// Debug: Check what time PostgreSQL thinks it is
	var dbNow string
	db.QueryRow("SELECT NOW()::TEXT").Scan(&dbNow)
	log.Printf("Database NOW(): %s", dbNow)

	// Debug: Check if session exists at all
	var existsCheck bool
	var storedExpireTime string
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1), (SELECT expire_time::TEXT FROM sessions WHERE id = $1)", sessionID).Scan(&existsCheck, &storedExpireTime)
	if err == nil {
		log.Printf("Session exists check: %v, Stored expire_time: %s", existsCheck, storedExpireTime)
	}

	query := `
		SELECT id, create_time, expire_time, index_val, name_id, user_email, user_common_name, groups
		FROM sessions
		WHERE id = $1 AND expire_time > NOW()
	`
	var session saml.Session
	var groups []string
	err = db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.CreateTime,
		&session.ExpireTime,
		&session.Index,
		&session.NameID,
		&session.UserEmail,
		&session.UserCommonName,
		pq.Array(&groups),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Session not found in database: ID=%s", sessionID)
		} else {
			log.Printf("Error retrieving session from database: %v", err)
		}
		return nil
	}
	session.Groups = groups
	log.Printf("Session retrieved successfully from database: ID=%s, Email=%s", session.ID, session.UserEmail)
	return &session
}

// cleanupExpiredSessions removes expired sessions from the database
func cleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expire_time < NOW()`
	_, err := db.Exec(query)
	return err
}

// saveServiceProviderToDB saves a service provider to the database
func saveServiceProviderToDB(entityID, acsURL, acsBinding string) error {
	log.Printf("Saving service provider to database: EntityID=%s, ACS URL=%s", entityID, acsURL)
	query := `
		INSERT INTO service_providers (entity_id, acs_url, acs_binding)
		VALUES ($1, $2, $3)
		ON CONFLICT (entity_id) DO UPDATE SET
			acs_url = EXCLUDED.acs_url,
			acs_binding = EXCLUDED.acs_binding
	`
	_, err := db.Exec(query, entityID, acsURL, acsBinding)
	if err != nil {
		log.Printf("Error saving service provider to database: %v", err)
	} else {
		log.Printf("Service provider saved successfully: EntityID=%s", entityID)
	}
	return err
}

// getServiceProviderFromDB retrieves a service provider from the database by entity ID
func getServiceProviderFromDB(entityID string) (*saml.EntityDescriptor, error) {
	log.Printf("Retrieving service provider from database: EntityID=%s", entityID)
	query := `
		SELECT entity_id, acs_url, acs_binding
		FROM service_providers
		WHERE entity_id = $1
	`
	var acsURL, acsBinding string
	var retrievedEntityID string
	err := db.QueryRow(query, entityID).Scan(&retrievedEntityID, &acsURL, &acsBinding)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Service provider not found in database: EntityID=%s", entityID)
		} else {
			log.Printf("Error retrieving service provider from database: %v", err)
		}
		return nil, err
	}

	log.Printf("Service provider retrieved successfully: EntityID=%s, ACS URL=%s", retrievedEntityID, acsURL)
	return &saml.EntityDescriptor{
		EntityID: retrievedEntityID,
		SPSSODescriptors: []saml.SPSSODescriptor{
			{
				AssertionConsumerServices: []saml.IndexedEndpoint{
					{
						Binding:  acsBinding,
						Location: acsURL,
						Index:    1,
					},
				},
			},
		},
	}, nil
}
