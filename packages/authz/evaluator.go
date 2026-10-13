package authz

import "strings"

// Can evaluates IAM-style policies for the principal, action, and resource.
// Default deny; explicit Deny beats Allow.
func Can(p Principal, action Action, res Resource) bool {
	policies := managedPolicies()
	var allowed, denied bool

	for _, roleKey := range p.effectiveRoleKeys(res) {
		pol, ok := policies[roleKey]
		if !ok {
			continue
		}
		for _, stmt := range pol.Statements {
			if !matchAction(stmt.Actions, action) {
				continue
			}
			if !matchResource(stmt.Resources, res) {
				continue
			}
			if !evalConditions(stmt.Condition, p, res) {
				continue
			}
			if stmt.Effect == EffectDeny {
				denied = true
			} else {
				allowed = true
			}
		}
	}

	if denied {
		return false
	}
	return allowed
}

func evalConditions(cond map[string]string, p Principal, res Resource) bool {
	if len(cond) == 0 {
		return true
	}
	for k, v := range cond {
		switch k {
		case "withoutPlatformRole":
			for _, role := range strings.Split(v, ",") {
				if contains(p.resolvedPlatformRoles(), role) {
					return false
				}
			}
		case "resourceOwner":
			isSelf := res.OwnerUserID > 0 && res.OwnerUserID == p.UserID
			switch v {
			case "self":
				if !isSelf {
					return false
				}
			case "notSelf":
				if isSelf {
					return false
				}
			default:
				return false
			}
		case "inScope":
			switch v {
			case "class":
				if p.ClassID <= 0 || res.ClassID <= 0 || p.ClassID != res.ClassID {
					return false
				}
			case "org":
				// The actor must hold an org role in the organization that owns the resource.
				if res.OrgID <= 0 || len(p.OrgRoleKeys(res.OrgID)) == 0 {
					return false
				}
			default:
				return false
			}
		default:
			return false
		}
	}
	return true
}

// EffectiveActions returns all actions allowed for the principal in the given resource context.
func EffectiveActions(p Principal, res Resource) []Action {
	all := []Action{
		ActionStudySetCreate, ActionStudySetRead, ActionStudySetUpdate, ActionStudySetDelete,
		ActionStudySetPublish, ActionStudySetImport, ActionStudySetExport, ActionStudySetSetPrice,
		ActionQuizAttempt, ActionQuizViewAnswers, ActionQuizViewResults,
		ActionClassCreate, ActionClassRead, ActionClassUpdate, ActionClassDelete,
		ActionClassResetInvite, ActionClassJoin, ActionClassViewDashboard,
		ActionClassMemberList, ActionClassMemberAdd, ActionClassMemberRemove,
		ActionClassMemberUpdateRole, ActionClassMemberViewProgress,
		ActionClassContentAssign, ActionClassContentUnassign, ActionClassContentUpload, ActionClassContentView,
		ActionLiveHost, ActionLiveJoin,
		ActionWalletRead, ActionOrderCreate, ActionOrderListAll, ActionWalletCredit,
		ActionUserReadSelf, ActionUserUpdateSelf, ActionUserList, ActionUserDisable,
		ActionIAMListRoles, ActionIAMListAssignments, ActionIAMAssignRole, ActionIAMRevokeRole,
		ActionDashboardViewSelf, ActionDashboardViewAdmin, ActionAuditRead,
		ActionOrgCreate, ActionOrgRead, ActionOrgUpdate, ActionOrgInviteMember, ActionOrgViewDashboard,
	}
	out := make([]Action, 0, len(all))
	for _, a := range all {
		if Can(p, a, res) {
			out = append(out, a)
		}
	}
	return out
}
