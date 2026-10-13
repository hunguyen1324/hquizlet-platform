package authz

import (
	"strconv"
	"strings"
)

// Principal is the actor requesting access.
type Principal struct {
	UserID        int64
	LegacyRole    string   // users.role: user | admin (compat)
	PlatformRoles []string // Teacher, Student, PlatformAdmin, ...
	ClassRole     string   // owner | teacher | student for the current class context
	ClassID       int64
	// OrgRoles holds the organization-scoped roles the actor holds, keyed by organization id.
	// A user may belong to several organizations with different roles in each.
	OrgRoles map[int64][]string
}

// ParsePlatformRoles parses a comma-separated X-User-Roles header value.
func ParsePlatformRoles(header string) []string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseOrgRoles parses the wire format "<orgID>:<RoleKey>" entries separated by commas,
// e.g. "7:OrgAdmin,9:OrgTeacher". Unparseable entries are skipped rather than failing the
// whole header, so one bad entry can never silently drop every other grant.
func ParseOrgRoles(header string) map[int64][]string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}
	out := make(map[int64][]string)
	for _, entry := range strings.Split(header, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		idx := strings.Index(entry, ":")
		if idx <= 0 || idx == len(entry)-1 {
			continue
		}
		orgID, err := strconv.ParseInt(strings.TrimSpace(entry[:idx]), 10, 64)
		if err != nil || orgID <= 0 {
			continue
		}
		role := strings.TrimSpace(entry[idx+1:])
		if role == "" {
			continue
		}
		if !contains(out[orgID], role) {
			out[orgID] = append(out[orgID], role)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// FormatOrgRole returns the canonical "orgID:RoleKey" wire entry.
func FormatOrgRole(orgID int64, roleKey string) string {
	return strconv.FormatInt(orgID, 10) + ":" + roleKey
}

// OrgRoleKeys returns the roles the actor holds inside one organization.
func (p Principal) OrgRoleKeys(orgID int64) []string {
	if orgID <= 0 || len(p.OrgRoles) == 0 {
		return nil
	}
	return p.OrgRoles[orgID]
}

func (p Principal) resolvedPlatformRoles() []string {
	roles := append([]string{}, p.PlatformRoles...)
	if p.LegacyRole == "admin" && !contains(roles, RolePlatformAdmin) {
		roles = append(roles, RolePlatformAdmin)
	}
	if len(roles) == 0 && p.LegacyRole == "user" {
		roles = append(roles, RoleStudent)
	}
	return roles
}

func (p Principal) effectiveRoleKeys(res Resource) []string {
	keys := append([]string{}, p.resolvedPlatformRoles()...)
	if cr := ClassMemberRole(p.ClassRole); cr != "" {
		if res.ClassID <= 0 || res.ClassID == p.ClassID {
			keys = append(keys, cr)
		}
	}
	// Org roles apply to the org itself and are inherited by every class of that org.
	keys = append(keys, p.OrgRoleKeys(res.OrgID)...)
	return keys
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
