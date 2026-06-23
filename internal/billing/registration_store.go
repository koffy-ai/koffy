package billing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type phoneVerificationRecord struct {
	ID        int64
	CodeHash  string
	Salt      string
	ExpiresAt time.Time
	Attempts  int
}

func (s *Store) CreatePhoneVerificationCode(ctx context.Context, phone string, purpose string, code string, secret string, now time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer rollback(tx)

	var latest time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT created_at
		FROM phone_verification_codes
		WHERE phone = ? AND purpose = ? AND consumed_at IS NULL
		ORDER BY id DESC
		LIMIT 1`,
		phone, purpose,
	).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && now.Sub(latest) < time.Minute {
		return "", ErrVerificationTooSoon
	}

	salt := randomHex(16)
	codeHash := hashVerificationCode(phone, code, salt, secret)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO phone_verification_codes (phone, purpose, code_hash, salt, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		phone,
		purpose,
		codeHash,
		salt,
		now.Add(10*time.Minute),
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return codeHash, nil
}

func (s *Store) ConsumePhoneVerificationCode(ctx context.Context, phone string, purpose string, code string, secret string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	var item phoneVerificationRecord
	if err := tx.QueryRowContext(ctx, `
		SELECT id, code_hash, salt, expires_at, attempts
		FROM phone_verification_codes
		WHERE phone = ? AND purpose = ? AND consumed_at IS NULL
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE`,
		phone,
		purpose,
	).Scan(&item.ID, &item.CodeHash, &item.Salt, &item.ExpiresAt, &item.Attempts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrVerificationInvalid
		}
		return err
	}
	if now.After(item.ExpiresAt) {
		return ErrVerificationExpired
	}
	if item.Attempts >= 5 {
		return ErrVerificationLocked
	}
	if hashVerificationCode(phone, code, item.Salt, secret) != item.CodeHash {
		_, _ = tx.ExecContext(ctx, `UPDATE phone_verification_codes SET attempts = attempts + 1 WHERE id = ?`, item.ID)
		if err := tx.Commit(); err != nil {
			return err
		}
		return ErrVerificationInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE phone_verification_codes SET consumed_at = ? WHERE id = ?`, now, item.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EnsureRegisteredUser(ctx context.Context, casdoorUserID string, owner string, phone string, displayName string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	if displayName == "" {
		displayName = maskPhone(phone)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (casdoor_user_id, casdoor_owner, name, display_name, phone, is_admin)
		VALUES (?, ?, ?, ?, ?, 0)
		ON DUPLICATE KEY UPDATE
			casdoor_owner = VALUES(casdoor_owner),
			name = VALUES(name),
			display_name = VALUES(display_name),
			phone = VALUES(phone)`,
		casdoorUserID,
		owner,
		casdoorUserID,
		displayName,
		phone,
	); err != nil {
		return err
	}
	user, err := findUserProfileForUpdate(ctx, tx, casdoorUserID)
	if err != nil {
		return err
	}
	if err := ensureWallet(ctx, tx, user.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UserNameExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE name = ? OR casdoor_user_id = ?)`,
		name,
		name,
	).Scan(&exists)
	return exists, err
}

func hashVerificationCode(phone string, code string, salt string, secret string) string {
	sum := sha256.Sum256([]byte(phone + "|" + code + "|" + salt + "|" + secret))
	return hex.EncodeToString(sum[:])
}
