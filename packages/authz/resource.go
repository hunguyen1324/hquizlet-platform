package authz

import (
	"strconv"
	"strings"
)

// Resource describes the target of an authorization check (ARN-style).
type Resource struct {
	Type        string // study-set, class, user, org, platform
	ID          int64
	OwnerUserID int64 // for study-set / user-owned resources
	ClassID     int64 // when checking class-scoped actions
	OrgID       int64 // organization a class belongs to (0 = personal class)
}

func (r Resource) ARN() string {
	if r.Type == "" {
		return "hq:*"
	}
	if r.ID <= 0 {
		return "hq:" + r.Type + "/*"
	}
	return "hq:" + r.Type + "/" + strconv.FormatInt(r.ID, 10)
}

func matchResource(patterns []string, res Resource) bool {
	arn := res.ARN()
	for _, p := range patterns {
		if p == "hq:*" || p == "*" {
			return true
		}
		if strings.HasSuffix(p, "/*") {
			prefix := strings.TrimSuffix(p, "/*")
			if strings.HasPrefix(arn, prefix+"/") || arn == prefix+"/*" {
				return true
			}
			continue
		}
		if p == arn {
			return true
		}
	}
	return false
}

func matchAction(patterns []string, action Action) bool {
	a := string(action)
	for _, p := range patterns {
		if p == "*" || p == a {
			return true
		}
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, ":") + ":"
			if strings.HasPrefix(a, prefix) {
				return true
			}
		}
	}
	return false
}
