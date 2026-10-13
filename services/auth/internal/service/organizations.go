package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
)

// OrganizationView is the admin-console view of an organization.
type OrganizationView struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Kind        string    `json:"kind"`
	OwnerUserID int64     `json:"ownerUserId"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CreateOrganizationInput is the body of POST /v1/admin/organizations.
type CreateOrganizationInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Kind        string `json:"kind"`
	OwnerUserID int64  `json:"ownerUserId"`
}

const maxOrganizationNameLen = 120

// ListOrganizations returns organizations for the admin console.
// Requires org:Read (PlatformAdmin holds it; OrgAdmin only inside its own org).
func (s *AuthService) ListOrganizations(ctx context.Context, actor authz.Principal, limit, offset int, requestID, ip string) ([]OrganizationView, int, error) {
	if err := requirePlatform(actor, authz.ActionOrgRead); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionOrgRead), "hq:org/*", requestID, ip)
		return nil, 0, err
	}
	rows, total, err := s.repo.ListOrganizations(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]OrganizationView, 0, len(rows))
	for _, o := range rows {
		out = append(out, organizationView(o))
	}
	return out, total, nil
}

// CreateOrganization creates an organization and audits the action.
// Only platform admins may create one: creating an org is a platform-scope decision, so
// org:Create is deliberately not granted to OrgAdmin.
func (s *AuthService) CreateOrganization(ctx context.Context, actor authz.Principal, in CreateOrganizationInput, requestID, ip string) (OrganizationView, error) {
	if err := requirePlatform(actor, authz.ActionOrgCreate); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionOrgCreate), "hq:org/*", requestID, ip)
		return OrganizationView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > maxOrganizationNameLen {
		return OrganizationView{}, &ValidationError{Field: "name", Msg: "name is required (max 120 characters)"}
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "school"
	}
	switch kind {
	case "school", "company", "other":
	default:
		return OrganizationView{}, &ValidationError{Field: "kind", Msg: "kind must be school, company or other"}
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = SlugifyOrgName(name)
	}
	if slug == "" {
		return OrganizationView{}, &ValidationError{Field: "slug", Msg: "slug is required"}
	}
	owner := in.OwnerUserID
	if owner <= 0 {
		owner = actor.UserID
	}

	created, err := s.repo.CreateOrganization(ctx, name, slug, kind, owner)
	if err != nil {
		return OrganizationView{}, err
	}
	_ = s.repo.InsertAuditLog(ctx, actor.UserID, string(authz.ActionOrgCreate), "hq:org/"+itoa(created.ID), "allow", requestID, ip, map[string]any{
		"name": created.Name, "slug": created.Slug, "kind": created.Kind,
	})
	return organizationView(created), nil
}

// validateOrgScopeAssignment enforces that org-scoped roles and org scope always come together.
// Returns a validation error rather than trusting the caller's shape.
func (s *AuthService) validateOrgScopeAssignment(ctx context.Context, roleKey, scopeType string, scopeID *int64) error {
	orgRole := isOrgScopedRole(roleKey)
	if orgRole && scopeType != "org" {
		return &ValidationError{Field: "scopeType", Msg: "role " + roleKey + " must be assigned at org scope"}
	}
	if scopeType != "org" {
		if orgRole {
			return &ValidationError{Field: "scopeType", Msg: "role " + roleKey + " must be assigned at org scope"}
		}
		return nil
	}
	if !orgRole {
		return &ValidationError{Field: "roleKey", Msg: "role " + roleKey + " is not an organization role"}
	}
	if scopeID == nil || *scopeID <= 0 {
		return &ValidationError{Field: "scopeId", Msg: "scopeId is required for org scope"}
	}
	exists, err := s.repo.OrganizationExists(ctx, *scopeID)
	if err != nil {
		return err
	}
	if !exists {
		return &ValidationError{Field: "scopeId", Msg: "organization not found"}
	}
	return nil
}

func isOrgScopedRole(roleKey string) bool {
	for _, r := range authz.OrgScopedRoleKeys() {
		if r == roleKey {
			return true
		}
	}
	return false
}

func organizationView(o repository.Organization) OrganizationView {
	return OrganizationView{ID: o.ID, Name: o.Name, Slug: o.Slug, Kind: o.Kind, OwnerUserID: o.OwnerUserID, CreatedAt: o.CreatedAt}
}

// SlugifyOrgName builds a URL-safe slug from a display name (lowercase ASCII + dashes).
func SlugifyOrgName(name string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Vietnamese diacritics are transliterated so the slug stays ASCII.
			if ascii := transliterate(r); ascii != 0 {
				b.WriteRune(ascii)
			} else if r < 128 {
				b.WriteRune(r)
			}
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

var vietnameseFold = map[rune]rune{
	'à': 'a', 'á': 'a', 'ả': 'a', 'ã': 'a', 'ạ': 'a', 'ă': 'a', 'ằ': 'a', 'ắ': 'a', 'ẳ': 'a', 'ẵ': 'a', 'ặ': 'a',
	'â': 'a', 'ầ': 'a', 'ấ': 'a', 'ẩ': 'a', 'ẫ': 'a', 'ậ': 'a',
	'đ': 'd',
	'è': 'e', 'é': 'e', 'ẻ': 'e', 'ẽ': 'e', 'ẹ': 'e', 'ê': 'e', 'ề': 'e', 'ế': 'e', 'ể': 'e', 'ễ': 'e', 'ệ': 'e',
	'ì': 'i', 'í': 'i', 'ỉ': 'i', 'ĩ': 'i', 'ị': 'i',
	'ò': 'o', 'ó': 'o', 'ỏ': 'o', 'õ': 'o', 'ọ': 'o', 'ô': 'o', 'ồ': 'o', 'ố': 'o', 'ổ': 'o', 'ỗ': 'o', 'ộ': 'o',
	'ơ': 'o', 'ờ': 'o', 'ớ': 'o', 'ở': 'o', 'ỡ': 'o', 'ợ': 'o',
	'ù': 'u', 'ú': 'u', 'ủ': 'u', 'ũ': 'u', 'ụ': 'u', 'ư': 'u', 'ừ': 'u', 'ứ': 'u', 'ử': 'u', 'ữ': 'u', 'ự': 'u',
	'ỳ': 'y', 'ý': 'y', 'ỷ': 'y', 'ỹ': 'y', 'ỵ': 'y',
}

func transliterate(r rune) rune {
	if ascii, ok := vietnameseFold[r]; ok {
		return ascii
	}
	return 0
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
