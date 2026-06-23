package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (s *Store) EnsureLocalCasdoorOAuth(ctx context.Context, organization, appName, clientID, clientSecret, redirectURI string) error {
	if organization == "" || appName == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		return fmt.Errorf("organization, app name, client id, client secret and redirect uri are required")
	}
	var appOwner string
	if err := s.db.QueryRowContext(ctx, `
		SELECT owner
		FROM casdoor.application
		WHERE name = ?
			AND (owner = ? OR organization = ?)
		ORDER BY owner = ? DESC
		LIMIT 1`,
		appName,
		organization,
		organization,
		organization,
	).Scan(&appOwner); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("casdoor application %s for organization %s was not found", appName, organization)
		}
		return err
	}

	redirects, err := json.Marshal([]string{redirectURI})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE casdoor.application
		SET client_id = ?,
			client_secret = ?,
			redirect_uris = ?,
			enable_password = 1,
			disable_signin = 0
		WHERE owner = ? AND name = ?`,
		clientID,
		clientSecret,
		string(redirects),
		appOwner,
		appName,
	)
	return err
}
