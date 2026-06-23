package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"koffy/internal/auth"
)

const (
	phonePurposeRegister       = "register"
	phonePurposeResetPassword  = "reset_password"
	phonePurposeBindPhone      = "bind_phone"
	phonePurposeChangePassword = "change_password"
)

type authState struct {
	State     string
	Provider  string
	AppType   string
	Action    string
	ReturnTo  string
	UserID    sql.NullInt64
	ExpiresAt time.Time
}

type wechatIdentity struct {
	AppType   string
	OpenID    string
	UnionID   string
	Nickname  string
	AvatarURL string
}

type AuthBindingSummary struct {
	PhoneBound           bool   `json:"phone_bound"`
	WechatBound          bool   `json:"wechat_bound"`
	WechatOfficialBound  bool   `json:"wechat_official_bound"`
	WechatNickname       string `json:"wechat_nickname"`
	WechatAvatarURL      string `json:"wechat_avatar_url"`
	WechatOfficialOpenID string `json:"-"`
}

type loginCodeRecord struct {
	Code      string
	UserID    int64
	ReturnTo  string
	ExpiresAt time.Time
}

type wechatPayOpenIDRecord struct {
	Code      string
	UserID    int64
	OpenID    string
	ExpiresAt time.Time
}

func (s *Store) CreateAuthState(ctx context.Context, state authState) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_states (state, provider, app_type, action, return_to, user_id, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		state.State,
		state.Provider,
		state.AppType,
		state.Action,
		state.ReturnTo,
		nullInt64Value(state.UserID),
		state.ExpiresAt,
	)
	return err
}

func (s *Store) ConsumeAuthState(ctx context.Context, state string, now time.Time) (authState, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return authState{}, err
	}
	defer rollback(tx)

	var item authState
	if err := tx.QueryRowContext(ctx, `
		SELECT state, provider, app_type, action, return_to, user_id, expires_at
		FROM auth_states
		WHERE state = ? AND consumed_at IS NULL
		FOR UPDATE`,
		state,
	).Scan(&item.State, &item.Provider, &item.AppType, &item.Action, &item.ReturnTo, &item.UserID, &item.ExpiresAt); err != nil {
		return authState{}, err
	}
	if now.After(item.ExpiresAt) {
		return authState{}, ErrVerificationExpired
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_states SET consumed_at = ? WHERE state = ?`, now, state); err != nil {
		return authState{}, err
	}
	return item, tx.Commit()
}

func (s *Store) AuthStateByState(ctx context.Context, state string) (authState, error) {
	var item authState
	err := s.db.QueryRowContext(ctx, `
		SELECT state, provider, app_type, action, return_to, user_id, expires_at
		FROM auth_states
		WHERE state = ?`,
		state,
	).Scan(&item.State, &item.Provider, &item.AppType, &item.Action, &item.ReturnTo, &item.UserID, &item.ExpiresAt)
	return item, err
}

func (s *Store) FindUserByPhone(ctx context.Context, phone string) (UserProfile, error) {
	var user UserProfile
	err := s.db.QueryRowContext(ctx, `
		SELECT id, casdoor_user_id, casdoor_owner, name, display_name, display_name_custom, email, phone, avatar_url, avatar_custom, is_admin, created_at, updated_at
		FROM users
		WHERE phone = ?
		ORDER BY id ASC
		LIMIT 1`,
		phone,
	).Scan(
		&user.ID,
		&user.CasdoorUserID,
		&user.Owner,
		&user.Name,
		&user.DisplayName,
		&user.DisplayCustom,
		&user.Email,
		&user.Phone,
		&user.AvatarURL,
		&user.AvatarCustom,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func (s *Store) UpdateUserPhone(ctx context.Context, userID int64, phone string, displayName string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET phone = ?, display_name = CASE WHEN display_name = '' THEN ? ELSE display_name END
		WHERE id = ?`,
		phone,
		displayName,
		userID,
	); err != nil {
		return err
	}

	var storedPhone string
	if err := tx.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&storedPhone); err != nil {
		return err
	}
	if !samePhoneValue(storedPhone, phone) {
		return fmt.Errorf("phone update was not persisted")
	}
	return tx.Commit()
}

func (s *Store) FindUserByWeChatIdentity(ctx context.Context, identity wechatIdentity) (UserProfile, error) {
	var user UserProfile
	args := []any{"wechat", identity.AppType, identity.OpenID}
	unionClause := ""
	if identity.UnionID != "" {
		unionClause = " OR (i.provider = ? AND i.unionid = ?)"
		args = append(args, "wechat", identity.UnionID)
	}
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.casdoor_user_id, u.casdoor_owner, u.name, u.display_name, u.display_name_custom, u.email, u.phone, u.avatar_url, u.avatar_custom, u.is_admin, u.created_at, u.updated_at
		FROM auth_identities i
		JOIN users u ON u.id = i.user_id
		WHERE (i.provider = ? AND i.app_type = ? AND i.openid = ?)`+unionClause+`
		ORDER BY i.id ASC
		LIMIT 1`,
		args...,
	).Scan(
		&user.ID,
		&user.CasdoorUserID,
		&user.Owner,
		&user.Name,
		&user.DisplayName,
		&user.DisplayCustom,
		&user.Email,
		&user.Phone,
		&user.AvatarURL,
		&user.AvatarCustom,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func (s *Store) BindWeChatIdentity(ctx context.Context, userID int64, identity wechatIdentity) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_identities (user_id, provider, app_type, openid, unionid, nickname, avatar_url)
		VALUES (?, 'wechat', ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			unionid = CASE WHEN VALUES(unionid) != '' THEN VALUES(unionid) ELSE unionid END,
			nickname = CASE WHEN VALUES(nickname) != '' THEN VALUES(nickname) ELSE nickname END,
			avatar_url = CASE WHEN VALUES(avatar_url) != '' THEN VALUES(avatar_url) ELSE avatar_url END`,
		userID,
		identity.AppType,
		identity.OpenID,
		identity.UnionID,
		identity.Nickname,
		identity.AvatarURL,
	)
	return err
}

func (s *Store) UnbindWeChatIdentity(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)

	var phone string
	if err := tx.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&phone); err != nil {
		return err
	}
	if phone == "" {
		return fmt.Errorf("请先绑定手机号，再解绑微信")
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM auth_identities WHERE user_id = ? AND provider = 'wechat'`, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Store) CreateWeChatPayOpenIDCode(ctx context.Context, userID int64, openID string, now time.Time) (string, error) {
	code, err := randomState()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO wechat_pay_openids (code, user_id, openid, expires_at)
		VALUES (?, ?, ?, ?)`,
		code,
		userID,
		openID,
		now.Add(5*time.Minute),
	)
	return code, err
}

func (s *Store) ConsumeWeChatPayOpenIDCode(ctx context.Context, userID int64, code string, now time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer rollback(tx)

	var item wechatPayOpenIDRecord
	if err := tx.QueryRowContext(ctx, `
		SELECT code, user_id, openid, expires_at
		FROM wechat_pay_openids
		WHERE code = ? AND consumed_at IS NULL
		FOR UPDATE`,
		code,
	).Scan(&item.Code, &item.UserID, &item.OpenID, &item.ExpiresAt); err != nil {
		return "", err
	}
	if item.UserID != userID {
		return "", ErrRequestConflict
	}
	if now.After(item.ExpiresAt) {
		return "", ErrVerificationExpired
	}
	if _, err := tx.ExecContext(ctx, `UPDATE wechat_pay_openids SET consumed_at = ? WHERE code = ?`, now, code); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return item.OpenID, nil
}

func (s *Store) ApplyWeChatProfileDefaults(ctx context.Context, userID int64, identity wechatIdentity) (UserProfile, bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return UserProfile{}, false, false, err
	}
	defer rollback(tx)

	user, err := findUserProfileByIDForUpdate(ctx, tx, userID)
	if err != nil {
		return UserProfile{}, false, false, err
	}
	displayChanged := false
	avatarChanged := false
	nextDisplayName := user.DisplayName
	nextAvatarURL := user.AvatarURL
	if !user.DisplayCustom && strings.TrimSpace(identity.Nickname) != "" && identity.Nickname != user.DisplayName {
		nextDisplayName, err = uniqueDisplayNameInTx(ctx, tx, identity.Nickname, userID)
		if err != nil {
			return UserProfile{}, false, false, err
		}
		displayChanged = true
	}
	if !user.AvatarCustom && strings.TrimSpace(identity.AvatarURL) != "" && identity.AvatarURL != user.AvatarURL {
		nextAvatarURL = strings.TrimSpace(identity.AvatarURL)
		avatarChanged = true
	}
	if displayChanged || avatarChanged {
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET display_name = ?, avatar_url = ?
			WHERE id = ?`,
			nextDisplayName,
			nextAvatarURL,
			userID,
		); err != nil {
			return UserProfile{}, false, false, err
		}
		user.DisplayName = nextDisplayName
		user.AvatarURL = nextAvatarURL
	}
	return user, displayChanged, avatarChanged, tx.Commit()
}

func (s *Store) EnsureWeChatUser(ctx context.Context, principal auth.Principal, identity wechatIdentity) (UserProfile, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return UserProfile{}, err
	}
	defer rollback(tx)

	phone := internalPhoneFromCasdoorPhone(principal.Phone)
	allowPhoneOverwrite := principalCanOverwriteStoredPhone(principal)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (
			casdoor_user_id, casdoor_owner, name, display_name, email, phone, avatar_url, is_admin, last_login_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			casdoor_owner = VALUES(casdoor_owner),
			name = VALUES(name),
			display_name = CASE
				WHEN display_name_custom THEN display_name
				WHEN VALUES(display_name) != '' THEN VALUES(display_name)
				ELSE display_name
			END,
			email = VALUES(email),
			phone = CASE
				WHEN VALUES(phone) = '' THEN phone
				WHEN ? THEN VALUES(phone)
				WHEN phone = '' THEN VALUES(phone)
				ELSE phone
			END,
			avatar_url = CASE
				WHEN avatar_custom THEN avatar_url
				WHEN VALUES(avatar_url) != '' THEN VALUES(avatar_url)
				ELSE avatar_url
			END,
			is_admin = VALUES(is_admin),
			last_login_at = CURRENT_TIMESTAMP(3)`,
		principal.ID,
		principal.Owner,
		firstNonEmpty(principal.Name, principal.ID),
		principal.DisplayName,
		principal.Email,
		phone,
		identity.AvatarURL,
		principal.IsAdmin,
		allowPhoneOverwrite,
	); err != nil {
		return UserProfile{}, err
	}
	user, err := findUserProfileForUpdate(ctx, tx, principal.ID)
	if err != nil {
		return UserProfile{}, err
	}
	if err := ensureWallet(ctx, tx, user.ID); err != nil {
		return UserProfile{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_identities (user_id, provider, app_type, openid, unionid, nickname, avatar_url)
		VALUES (?, 'wechat', ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			unionid = VALUES(unionid),
			nickname = VALUES(nickname),
			avatar_url = VALUES(avatar_url)`,
		user.ID,
		identity.AppType,
		identity.OpenID,
		identity.UnionID,
		identity.Nickname,
		identity.AvatarURL,
	); err != nil {
		return UserProfile{}, err
	}
	return user, tx.Commit()
}

func (s *Store) AuthBindings(ctx context.Context, userID int64) (AuthBindingSummary, error) {
	var result AuthBindingSummary
	var phone string
	if err := s.db.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = ?`, userID).Scan(&phone); err != nil {
		return result, err
	}
	result.PhoneBound = phone != ""
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM auth_identities WHERE user_id = ? AND provider = 'wechat')`,
		userID,
	).Scan(&result.WechatBound)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return result, err
	}
	var appType string
	err = s.db.QueryRowContext(ctx, `
		SELECT app_type, nickname, avatar_url, openid
		FROM auth_identities
		WHERE user_id = ? AND provider = 'wechat'
		ORDER BY app_type = 'official' DESC, id DESC
		LIMIT 1`,
		userID,
	).Scan(&appType, &result.WechatNickname, &result.WechatAvatarURL, &result.WechatOfficialOpenID)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return result, err
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM auth_identities WHERE user_id = ? AND provider = 'wechat' AND app_type = 'official')`,
		userID,
	).Scan(&result.WechatOfficialBound)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return result, err
}

func (s *Store) CreateLoginCode(ctx context.Context, userID int64, returnTo string, now time.Time) (string, error) {
	code, err := randomState()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO auth_login_codes (code, user_id, return_to, expires_at)
		VALUES (?, ?, ?, ?)`,
		code,
		userID,
		returnTo,
		now.Add(2*time.Minute),
	)
	return code, err
}

func (s *Store) ConsumeLoginCode(ctx context.Context, code string, now time.Time) (UserProfile, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return UserProfile{}, err
	}
	defer rollback(tx)

	var item loginCodeRecord
	if err := tx.QueryRowContext(ctx, `
		SELECT code, user_id, return_to, expires_at
		FROM auth_login_codes
		WHERE code = ? AND consumed_at IS NULL
		FOR UPDATE`,
		code,
	).Scan(&item.Code, &item.UserID, &item.ReturnTo, &item.ExpiresAt); err != nil {
		return UserProfile{}, err
	}
	if now.After(item.ExpiresAt) {
		return UserProfile{}, ErrVerificationExpired
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_login_codes SET consumed_at = ? WHERE code = ?`, now, code); err != nil {
		return UserProfile{}, err
	}
	user, err := findUserProfileByIDForUpdate(ctx, tx, item.UserID)
	if err != nil {
		return UserProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return UserProfile{}, err
	}
	return user, nil
}

func nullInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
