package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

// Use PostgreSQL-style $1, $2 placeholders.
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// SessionRepo is the PostgreSQL implementation of repository.SessionRepository.
type SessionRepo struct {
	db DBTX
}

// NewSessionRepo creates a new SessionRepo backed by the given DBTX
// (either a *pgxpool.Pool or a pgx.Tx).
func NewSessionRepo(db DBTX) *SessionRepo {
	return &SessionRepo{db: db}
}

// Save persists a session, upserting on conflict by ID.
func (r *SessionRepo) Save(ctx context.Context, session *domain.Session) error {
	// Serialize raw OIDC claims to JSONB (nil-safe).
	var claimsJSON interface{}
	if session.RawOIDCClaims != nil {
		data, err := json.Marshal(session.RawOIDCClaims)
		if err != nil {
			return fmt.Errorf("marshal raw OIDC claims: %w", err)
		}
		claimsJSON = data
	}

	query, args, err := psql.
		Insert("sessions").
		Columns(
			"id", "create_time", "expire_time", "index_val",
			"name_id", "user_email", "user_common_name", "groups",
			"user_name", "raw_oidc_claims",
		).
		Values(
			session.ID, session.CreateTime, session.ExpireTime, session.Index,
			session.NameID, session.UserEmail, session.UserCommonName,
			session.Groups, session.UserName, claimsJSON,
		).
		Suffix(`ON CONFLICT (id) DO UPDATE SET
			create_time      = EXCLUDED.create_time,
			expire_time      = EXCLUDED.expire_time,
			index_val        = EXCLUDED.index_val,
			name_id          = EXCLUDED.name_id,
			user_email       = EXCLUDED.user_email,
			user_common_name = EXCLUDED.user_common_name,
			groups           = EXCLUDED.groups,
			user_name        = EXCLUDED.user_name,
			raw_oidc_claims  = EXCLUDED.raw_oidc_claims`).
		ToSql()
	if err != nil {
		return fmt.Errorf("build save session query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec save session: %w", err)
	}
	return nil
}

// GetByID retrieves a non-expired session by its ID.
// Returns *domain.ErrNotFound if no matching session exists.
func (r *SessionRepo) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	query, args, err := psql.
		Select(
			"id", "create_time", "expire_time", "index_val",
			"name_id", "user_email", "user_common_name", "groups",
			"user_name", "raw_oidc_claims",
		).
		From("sessions").
		Where(sq.Eq{"id": id}).
		Where("expire_time > NOW()").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get session query: %w", err)
	}

	var session domain.Session
	var claimsJSON *string
	err = r.db.QueryRow(ctx, query, args...).Scan(
		&session.ID, &session.CreateTime, &session.ExpireTime, &session.Index,
		&session.NameID, &session.UserEmail, &session.UserCommonName,
		&session.Groups, &session.UserName, &claimsJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &domain.ErrNotFound{Resource: "session", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("scan session %s: %w", id, err)
	}

	// Deserialize raw OIDC claims if present.
	if claimsJSON != nil && *claimsJSON != "" {
		if err := json.Unmarshal([]byte(*claimsJSON), &session.RawOIDCClaims); err != nil {
			return nil, fmt.Errorf("unmarshal raw OIDC claims for session %s: %w", id, err)
		}
	}

	return &session, nil
}

// DeleteExpired removes all sessions whose expire_time is in the past
// and returns the number of deleted rows.
func (r *SessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query, args, err := psql.
		Delete("sessions").
		Where("expire_time < NOW()").
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build delete expired query: %w", err)
	}

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("exec delete expired: %w", err)
	}
	return tag.RowsAffected(), nil
}
