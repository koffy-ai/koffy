package billing

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	var req updateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if len([]rune(displayName)) < 2 || len([]rune(displayName)) > 30 {
		httpx.Error(w, http.StatusBadRequest, "invalid_display_name", "昵称需为 2 到 30 个字符")
		return
	}
	if exists, err := s.store.DisplayNameExists(r.Context(), displayName, current.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	} else if exists {
		httpx.Error(w, http.StatusConflict, "display_name_exists", "该昵称已被使用")
		return
	}

	casdoorUser, ok := s.currentWritableCasdoorUser(w, current, "当前账号不属于 Koffy 认证组织，不能在这里修改个人资料")
	if !ok {
		return
	}
	casdoorUser.DisplayName = displayName
	if ok, err := casdoorsdk.UpdateUserForColumns(casdoorUser, []string{"displayName"}); err != nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", err.Error())
		return
	} else if !ok {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", "认证服务未更新用户资料")
		return
	}
	if err := s.store.UpdateUserDisplayName(r.Context(), current.ID, displayName); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	updated, err := s.store.UserByCasdoorID(r.Context(), current.CasdoorUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (s *Server) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	current, ok := s.currentUserProfile(w, r)
	if !ok {
		return
	}
	if !s.ensureUserInConfiguredCasdoorOrganization(w, current, "当前账号不属于 Koffy 认证组织，不能在这里修改头像") {
		return
	}
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_upload", "上传文件无效")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "missing_avatar", "请选择头像图片")
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 24<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_upload", "读取头像失败")
		return
	}
	asset, err := processAvatarUpload(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_avatar", err.Error())
		return
	}

	avatarURL := s.publicAvatarURL(current.CasdoorUserID)
	if err := s.store.SaveUserAvatarAsset(r.Context(), current.ID, avatarURL, asset); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	casdoorUser, ok := s.currentWritableCasdoorUser(w, current, "当前账号不属于 Koffy 认证组织，不能在这里修改头像")
	if !ok {
		return
	}
	casdoorUser.Avatar = avatarURL
	if ok, err := casdoorsdk.UpdateUserForColumns(casdoorUser, []string{"avatar"}); err != nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", err.Error())
		return
	} else if !ok {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_update_failed", "认证服务未更新头像")
		return
	}

	updated, err := s.store.UserByCasdoorID(r.Context(), current.CasdoorUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (s *Server) publicUserAvatar(w http.ResponseWriter, r *http.Request) {
	casdoorUserID := r.PathValue("casdoor_user_id")
	asset, err := s.store.PublicAvatarAsset(r.Context(), casdoorUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	_, _ = w.Write(asset.Data)
}

func (s *Server) publicAvatarURL(casdoorUserID string) string {
	return strings.TrimRight(s.cfg.PublicWebURL, "/") + "/api/v1/users/avatar/" + url.PathEscape(casdoorUserID)
}
