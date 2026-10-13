package authz

// Managed role keys (platform + class + organization scope).
const (
	RolePlatformAdmin   = "PlatformAdmin"
	RolePlatformSupport = "PlatformSupport"
	RoleTeacher         = "Teacher"
	RoleStudent         = "Student"
	RoleClassOwner      = "ClassOwner"
	RoleClassTeacher    = "ClassTeacher"
	RoleClassStudent    = "ClassStudent"
	RoleOrgAdmin        = "OrgAdmin"
	RoleOrgTeacher      = "OrgTeacher"
	RoleOrgStudent      = "OrgStudent"
)

// OrgScopedRoleKeys returns the roles that are only meaningful inside an organization.
func OrgScopedRoleKeys() []string {
	return []string{RoleOrgAdmin, RoleOrgTeacher, RoleOrgStudent}
}

// ClassMemberRole maps class_members.role to IAM role keys.
func ClassMemberRole(memberRole string) string {
	switch memberRole {
	case "owner":
		return RoleClassOwner
	case "teacher":
		return RoleClassTeacher
	case "student":
		return RoleClassStudent
	default:
		return ""
	}
}

type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)

type Statement struct {
	Effect    Effect
	Actions   []string
	Resources []string
	Condition map[string]string
}

type Policy struct {
	Version    string
	Statements []Statement
}

func managedPolicies() map[string]Policy {
	allStudyOwn := []string{
		string(ActionStudySetCreate), string(ActionStudySetRead), string(ActionStudySetUpdate),
		string(ActionStudySetDelete), string(ActionStudySetPublish), string(ActionStudySetImport),
		string(ActionStudySetExport),
	}
	studyOwnAllow := Statement{
		Effect: EffectAllow, Actions: allStudyOwn,
		Resources: []string{"hq:study-set/*"}, Condition: map[string]string{"resourceOwner": "self"},
	}
	studyOwnDenyOthers := Statement{
		Effect: EffectDeny, Actions: allStudyOwn,
		Resources: []string{"hq:study-set/*"}, Condition: map[string]string{"resourceOwner": "notSelf", "withoutPlatformRole": RolePlatformAdmin},
	}
	denySetPrice := Statement{
		Effect: EffectDeny, Actions: []string{string(ActionStudySetSetPrice)},
		Resources: []string{"hq:study-set/*"}, Condition: map[string]string{"withoutPlatformRole": RoleTeacher + "," + RolePlatformAdmin},
	}

	return map[string]Policy{
		RolePlatformAdmin: {
			Version: "2026-10-01",
			Statements: []Statement{{
				Effect: EffectAllow, Actions: []string{"*"}, Resources: []string{"hq:*"},
			}},
		},
		RolePlatformSupport: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{string(ActionUserList), string(ActionClassRead), string(ActionOrderListAll), string(ActionAuditRead)}, Resources: []string{"hq:*"}},
			},
		},
		RoleTeacher: {
			Version: "2026-10-01",
			Statements: []Statement{
				studyOwnAllow, studyOwnDenyOthers,
				{Effect: EffectAllow, Actions: []string{string(ActionStudySetSetPrice)}, Resources: []string{"hq:study-set/*"}, Condition: map[string]string{"resourceOwner": "self"}},
				{Effect: EffectAllow, Actions: []string{string(ActionClassCreate), string(ActionClassJoin), string(ActionLiveHost), string(ActionDashboardViewSelf)}, Resources: []string{"hq:*"}},
				{Effect: EffectAllow, Actions: []string{string(ActionWalletRead), string(ActionOrderCreate)}, Resources: []string{"hq:*"}},
			},
		},
		RoleStudent: {
			Version: "2026-10-01",
			Statements: []Statement{
				studyOwnAllow, studyOwnDenyOthers, denySetPrice,
				{Effect: EffectAllow, Actions: []string{string(ActionClassJoin), string(ActionLiveJoin), string(ActionDashboardViewSelf), string(ActionQuizAttempt)}, Resources: []string{"hq:*"}},
				{Effect: EffectAllow, Actions: []string{string(ActionWalletRead), string(ActionOrderCreate)}, Resources: []string{"hq:*"}},
			},
		},
		RoleClassOwner: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassRead), string(ActionClassUpdate), string(ActionClassDelete),
					string(ActionClassResetInvite), string(ActionClassViewDashboard),
					string(ActionClassMemberList), string(ActionClassMemberAdd), string(ActionClassMemberRemove),
					string(ActionClassMemberUpdateRole), string(ActionClassMemberViewProgress),
					string(ActionClassContentAssign), string(ActionClassContentUnassign),
					string(ActionClassContentUpload), string(ActionClassContentView),
					string(ActionLiveHost),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "class"}},
			},
		},
		RoleClassTeacher: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassRead), string(ActionClassViewDashboard),
					string(ActionClassMemberList), string(ActionClassMemberAdd), string(ActionClassMemberRemove),
					string(ActionClassMemberUpdateRole), string(ActionClassMemberViewProgress),
					string(ActionClassContentAssign), string(ActionClassContentUnassign),
					string(ActionClassContentUpload), string(ActionClassContentView),
					string(ActionLiveHost),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "class"}},
				// A class teacher may adjust member roles but never owns the class, so the service
				// rejects any attempt to touch the owner (condition "resourceOwner" is not expressive
				// enough for a per-target check here).
				{Effect: EffectDeny, Actions: []string{string(ActionClassDelete), string(ActionClassResetInvite)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"withoutPlatformRole": RolePlatformAdmin, "inScope": "class"}},
			},
		},
		RoleClassStudent: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassRead), string(ActionClassViewDashboard), string(ActionClassMemberList),
					string(ActionClassContentView), string(ActionLiveJoin), string(ActionQuizAttempt),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "class"}},
				{Effect: EffectAllow, Actions: []string{string(ActionClassMemberViewProgress)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "class", "resourceOwner": "self"}},
				{Effect: EffectDeny, Actions: []string{string(ActionClassMemberViewProgress)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"withoutPlatformRole": RolePlatformAdmin, "inScope": "class", "resourceOwner": "notSelf"}},
			},
		},
		// --- Organization roles (Phase 5). Granted per org; the same policy then applies to
		// every class whose org_id is that org, because effectiveRoleKeys() adds them when
		// Resource.OrgID matches. No duplication of class policies is needed.
		RoleOrgAdmin: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{
					string(ActionOrgRead), string(ActionOrgUpdate), string(ActionOrgInviteMember), string(ActionOrgViewDashboard),
				}, Resources: []string{"hq:org/*"}, Condition: map[string]string{"inScope": "org"}},
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassCreate), string(ActionClassRead), string(ActionClassUpdate), string(ActionClassDelete),
					string(ActionClassResetInvite), string(ActionClassViewDashboard),
					string(ActionClassMemberList), string(ActionClassMemberAdd), string(ActionClassMemberRemove),
					string(ActionClassMemberUpdateRole), string(ActionClassMemberViewProgress),
					string(ActionClassContentAssign), string(ActionClassContentUnassign),
					string(ActionClassContentUpload), string(ActionClassContentView),
					string(ActionLiveHost),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "org"}},
			},
		},
		RoleOrgTeacher: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{string(ActionOrgRead), string(ActionOrgViewDashboard)}, Resources: []string{"hq:org/*"}, Condition: map[string]string{"inScope": "org"}},
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassRead), string(ActionClassViewDashboard),
					string(ActionClassMemberList), string(ActionClassMemberAdd), string(ActionClassMemberRemove),
					string(ActionClassMemberUpdateRole), string(ActionClassMemberViewProgress),
					string(ActionClassContentAssign), string(ActionClassContentUnassign),
					string(ActionClassContentUpload), string(ActionClassContentView),
					string(ActionLiveHost),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "org"}},
				// An org teacher never owns a class: destructive class actions stay denied.
				{Effect: EffectDeny, Actions: []string{string(ActionClassDelete), string(ActionClassResetInvite)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"withoutPlatformRole": RolePlatformAdmin, "inScope": "org"}},
			},
		},
		RoleOrgStudent: {
			Version: "2026-10-01",
			Statements: []Statement{
				{Effect: EffectAllow, Actions: []string{
					string(ActionClassRead), string(ActionClassViewDashboard), string(ActionClassMemberList),
					string(ActionClassContentView), string(ActionLiveJoin), string(ActionQuizAttempt),
				}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "org"}},
				{Effect: EffectAllow, Actions: []string{string(ActionClassMemberViewProgress)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"inScope": "org", "resourceOwner": "self"}},
				{Effect: EffectDeny, Actions: []string{string(ActionClassMemberViewProgress)}, Resources: []string{"hq:class/*"}, Condition: map[string]string{"withoutPlatformRole": RolePlatformAdmin, "inScope": "org", "resourceOwner": "notSelf"}},
			},
		},
	}
}

// ManagedRoleKeys returns platform + class + org managed role identifiers.
func ManagedRoleKeys() []string {
	return []string{
		RolePlatformAdmin, RolePlatformSupport, RoleTeacher, RoleStudent,
		RoleClassOwner, RoleClassTeacher, RoleClassStudent,
		RoleOrgAdmin, RoleOrgTeacher, RoleOrgStudent,
	}
}

// ManagedPolicy returns a fresh policy value for read-only role inspection.
// Runtime evaluation and the admin console therefore use the same definitions.
func ManagedPolicy(key string) (Policy, bool) {
	policy, ok := managedPolicies()[key]
	return policy, ok
}
