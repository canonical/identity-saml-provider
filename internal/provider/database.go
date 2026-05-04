package provider

import (
	"database/sql"
	"encoding/json"

	"github.com/crewjam/saml"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// Database wraps a sql.DB connection and provides SAML-specific operations.
//
// Deprecated: Database is retained for backward compatibility during the
// migration to the repository pattern. New code should use the repository
// interfaces in internal/repository and the PostgreSQL implementations
// in internal/repository/postgres instead.
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

// GetDB returns the underlying sql.DB connection
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// SaveSession saves a SAML session to the database along with raw OIDC claims.
func (d *Database) SaveSession(session *saml.Session, rawClaims map[string]interface{}) error {
	d.logger.Infow("Saving session to database", "sessionID", session.ID, "email", session.UserEmail, "expireTime", session.ExpireTime)

	var claimsArg interface{}
	if rawClaims != nil {
		claimsJSON, err := json.Marshal(rawClaims)
		if err != nil {
			return err
		}
		claimsArg = claimsJSON
	}

	query := `
		INSERT INTO sessions (id, create_time, expire_time, index_val, name_id, user_email, user_common_name, groups, user_name, raw_oidc_claims)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			create_time = EXCLUDED.create_time,
			expire_time = EXCLUDED.expire_time,
			index_val = EXCLUDED.index_val,
			name_id = EXCLUDED.name_id,
			user_email = EXCLUDED.user_email,
			user_common_name = EXCLUDED.user_common_name,
			groups = EXCLUDED.groups,
			user_name = EXCLUDED.user_name,
			raw_oidc_claims = EXCLUDED.raw_oidc_claims
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
		session.UserName,
		claimsArg,
	)
	if err != nil {
		d.logger.Errorw("Error saving session to database", "sessionID", session.ID, "error", err)
	} else {
		d.logger.Infow("Session saved successfully to database", "sessionID", session.ID)
	}
	return err
}

// GetSession retrieves a SAML session and its raw OIDC claims from the database by ID.
func (d *Database) GetSession(sessionID string) (*saml.Session, map[string]interface{}) {
	d.logger.Infow("Attempting to retrieve session from database", "sessionID", sessionID)

	query := `
		SELECT id, create_time, expire_time, index_val, name_id, user_email, user_common_name, groups, user_name, raw_oidc_claims
		FROM sessions
		WHERE id = $1 AND expire_time > NOW()
	`
	var session saml.Session
	var groups []string
	var claimsJSON sql.NullString
	err := d.db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.CreateTime,
		&session.ExpireTime,
		&session.Index,
		&session.NameID,
		&session.UserEmail,
		&session.UserCommonName,
		pq.Array(&groups),
		&session.UserName,
		&claimsJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			d.logger.Infow("Session not found in database", "sessionID", sessionID)
		} else {
			d.logger.Errorw("Error retrieving session from database", "sessionID", sessionID, "error", err)
		}
		return nil, nil
	}
	session.Groups = groups

	var rawClaims map[string]interface{}
	if claimsJSON.Valid && claimsJSON.String != "" {
		if err := json.Unmarshal([]byte(claimsJSON.String), &rawClaims); err != nil {
			d.logger.Errorw("Error parsing raw OIDC claims JSON", "sessionID", sessionID, "error", err)
		}
	}

	d.logger.Infow("Session retrieved successfully from database", "sessionID", session.ID, "email", session.UserEmail)
	return &session, rawClaims
}

// CleanupExpiredSessions removes expired sessions from the database
func (d *Database) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expire_time < NOW()`
	_, err := d.db.Exec(query)
	return err
}

// SaveServiceProvider saves a service provider to the database
func (d *Database) SaveServiceProvider(entityID, acsURL, acsBinding string, attributeMapping *AttributeMapping) error {
	d.logger.Infow("Saving service provider to database", "entityID", entityID, "acsURL", acsURL)

	var mappingArg interface{}
	if attributeMapping != nil {
		mappingJSON, err := json.Marshal(attributeMapping)
		if err != nil {
			return err
		}
		mappingArg = mappingJSON
	}

	query := `
		INSERT INTO service_providers (entity_id, acs_url, acs_binding, attribute_mapping)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entity_id) DO UPDATE SET
			acs_url = EXCLUDED.acs_url,
			acs_binding = EXCLUDED.acs_binding,
			attribute_mapping = EXCLUDED.attribute_mapping
	`
	_, err := d.db.Exec(query, entityID, acsURL, acsBinding, mappingArg)
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

// GetAttributeMapping retrieves the attribute mapping for a service provider by entity ID.
// Returns nil if no mapping is configured for the SP.
func (d *Database) GetAttributeMapping(entityID string) (*AttributeMapping, error) {
	query := `
		SELECT attribute_mapping
		FROM service_providers
		WHERE entity_id = $1
	`
	var mappingJSON sql.NullString
	err := d.db.QueryRow(query, entityID).Scan(&mappingJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if !mappingJSON.Valid || mappingJSON.String == "" {
		return nil, nil
	}
	var mapping AttributeMapping
	if err := json.Unmarshal([]byte(mappingJSON.String), &mapping); err != nil {
		d.logger.Errorw("Error parsing attribute mapping JSON", "entityID", entityID, "error", err)
		return nil, err
	}
	return &mapping, nil
}
