package billing

import "koffy/internal/auth"

func principalCanOverwriteStoredPhone(principal auth.Principal) bool {
	switch principal.Source {
	case auth.PrincipalSourceCasdoorJWT, auth.PrincipalSourceLocalJWT, auth.PrincipalSourceLocalHeader:
		return true
	default:
		return false
	}
}
