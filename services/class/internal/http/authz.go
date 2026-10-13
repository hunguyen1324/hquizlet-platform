package http

import (
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
)

func principalFromRequest(r *http.Request) authz.Principal {
	userID := userIDFromHeader(r)
	return authz.Principal{
		UserID:        userID,
		LegacyRole:    r.Header.Get("X-User-Role"),
		PlatformRoles: authz.ParsePlatformRoles(r.Header.Get("X-User-Roles")),
	}
}
