package http

import (
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
)

// principalFromRequest builds the IAM principal from identity headers injected by the
// gateway (signed with INTERNAL_SIGNING_KEY). Spoofed client headers are stripped upstream
// and by VerifyBearer, so these values are trustworthy inside the service.
func principalFromRequest(r *http.Request) authz.Principal {
	return authz.Principal{
		UserID:        userIDFromHeader(r),
		LegacyRole:    r.Header.Get("X-User-Role"),
		PlatformRoles: authz.ParsePlatformRoles(r.Header.Get("X-User-Roles")),
		OrgRoles:      authz.ParseOrgRoles(r.Header.Get("X-User-Org-Roles")),
	}
}
