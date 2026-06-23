package billing

import (
	"net/http"
	"strings"

	"koffy/internal/httpx"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

func (s *Server) ensureUserInConfiguredCasdoorOrganization(w http.ResponseWriter, user UserProfile, actionMessage string) bool {
	owner := strings.TrimSpace(user.Owner)
	expected := strings.TrimSpace(s.cfg.CasdoorOrganizationName)
	if owner == "" || expected == "" || strings.EqualFold(owner, expected) {
		return true
	}
	httpx.Error(w, http.StatusBadRequest, "casdoor_user_owner_mismatch", actionMessage)
	return false
}

func (s *Server) currentWritableCasdoorUser(w http.ResponseWriter, user UserProfile, actionMessage string) (*casdoorsdk.User, bool) {
	if !s.ensureUserInConfiguredCasdoorOrganization(w, user, actionMessage) {
		return nil, false
	}
	casdoorUser, err := casdoorsdk.GetUser(user.CasdoorUserID)
	if err != nil || casdoorUser == nil {
		httpx.Error(w, http.StatusBadGateway, "casdoor_user_not_found", "认证服务用户不存在")
		return nil, false
	}
	return casdoorUser, true
}
