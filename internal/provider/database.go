package provider

import (
	"database/sql"

	"github.com/crewjam/saml"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// Database wraps a sql.DB connection and provides SAML-specific operations
type Database struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

// NewDatabase creates a new Database instance
func NewDatabase(db *sql.DB, logger *zap.SugaredLogger) *Database {
	return &Database{
		db:     db,
		logger: logger,
	}
}

// InitSchema creates the sessions and service_providers tables if they don't exist
func (d *Database) InitSchema() error {
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
	_, err := d.db.Exec(query)
	if err != nil {
		return err
	}
	d.logger.Info("Database schema initialized")
	return nil
}

// SaveSession saves a SAML session to the database
func (d *Database) SaveSession(session *saml.Session) error {
	d.logger.Infow("Saving session to database", "sessionID", session.ID, "email", session.UserEmail, "expireTime", session.ExpireTime)
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
	_, err := d.db.Exec(query,
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
		d.logger.Errorw("Error saving session to database", "sessionID", session.ID, "error", err)
	} else {
		d.logger.Infow("Session saved successfully to database", "sessionID", session.ID)
	}
	return err
}

// GetSession retrieves a SAML session from the database by ID
func (d *Database) GetSession(sessionID string) *saml.Session {
	d.logger.Infow("Attempting to retrieve session from database", "sessionID", sessionID)

	query := `
		SELECT id, create_time, expire_time, index_val, name_id, user_email, user_common_name, groups
		FROM sessions
		WHERE id = $1 AND expire_time > NOW()
	`
	var session saml.Session
	var groups []string
	err := d.db.QueryRow(query, sessionID).Scan(
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
			d.logger.Infow("Session not found in database", "sessionID", sessionID)
		} else {
			d.logger.Errorw("Error retrieving session from database", "sessionID", sessionID, "error", err)
		}
		return nil
	}
	session.Groups = groups
	d.logger.Infow("Session retrieved successfully from database", "sessionID", session.ID, "email", session.UserEmail)
	return &session
}

// CleanupExpiredSessions removes expired sessions from the database
func (d *Database) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expire_time < NOW()`
	_, err := d.db.Exec(query)
	return err
}

// SaveServiceProvider saves a service provider to the database
func (d *Database) SaveServiceProvider(entityID, acsURL, acsBinding string) error {
	d.logger.Infow("Saving service provider to database", "entityID", entityID, "acsURL", acsURL)
	query := `
		INSERT INTO service_providers (entity_id, acs_url, acs_binding)
		VALUES ($1, $2, $3)
		ON CONFLICT (entity_id) DO UPDATE SET
			acs_url = EXCLUDED.acs_url,
			acs_binding = EXCLUDED.acs_binding
	`
	_, err := d.db.Exec(query, entityID, acsURL, acsBinding)
	if err != nil {
		d.logger.Errorw("Error saving service provider to database", "entityID", entityID, "error", err)
	} else {
		d.logger.Infow("Service provider saved successfully", "entityID", entityID)
	}
	return err
}

// GetServiceProvider retrieves a service provider from the database by entity ID
func (d *Database) GetServiceProvider(entityID string) (*saml.EntityDescriptor, error) {
	d.logger.Infow("Retrieving service provider from database", "entityID", entityID)
	query := `
		SELECT entity_id, acs_url, acs_binding
		FROM service_providers
		WHERE entity_id = $1
	`
	var acsURL, acsBinding string
	var retrievedEntityID string
	err := d.db.QueryRow(query, entityID).Scan(&retrievedEntityID, &acsURL, &acsBinding)
	if err != nil {
		if err == sql.ErrNoRows {
			d.logger.Infow("Service provider not found in database", "entityID", entityID)
		} else {
			d.logger.Errorw("Error retrieving service provider from database", "entityID", entityID, "error", err)
		}
		return nil, err
	}

	d.logger.Infow("Service provider retrieved successfully", "entityID", retrievedEntityID, "acsURL", acsURL)
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
