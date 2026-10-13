package authz

// Action names follow service:Verb (IAM-style).
type Action string

const (
	ActionStudySetCreate   Action = "study-set:Create"
	ActionStudySetRead     Action = "study-set:Read"
	ActionStudySetUpdate   Action = "study-set:Update"
	ActionStudySetDelete   Action = "study-set:Delete"
	ActionStudySetPublish  Action = "study-set:Publish"
	ActionStudySetImport   Action = "study-set:Import"
	ActionStudySetExport   Action = "study-set:Export"
	ActionStudySetSetPrice Action = "study-set:SetPrice"

	ActionQuizAttempt     Action = "quiz:Attempt"
	ActionQuizViewAnswers Action = "quiz:ViewAnswers"
	ActionQuizViewResults Action = "quiz:ViewResults"

	ActionClassCreate        Action = "class:Create"
	ActionClassRead          Action = "class:Read"
	ActionClassUpdate        Action = "class:Update"
	ActionClassDelete        Action = "class:Delete"
	ActionClassResetInvite   Action = "class:ResetInvite"
	ActionClassJoin          Action = "class:Join"
	ActionClassViewDashboard Action = "class:ViewDashboard"

	ActionClassMemberList         Action = "class-member:List"
	ActionClassMemberAdd          Action = "class-member:Add"
	ActionClassMemberRemove       Action = "class-member:Remove"
	ActionClassMemberUpdateRole   Action = "class-member:UpdateRole"
	ActionClassMemberViewProgress Action = "class-member:ViewProgress"

	ActionClassContentAssign   Action = "class-content:AssignStudySet"
	ActionClassContentUnassign Action = "class-content:UnassignStudySet"
	ActionClassContentUpload   Action = "class-content:UploadVideo"
	ActionClassContentView     Action = "class-content:ViewVideo"

	ActionLiveHost Action = "live:Host"
	ActionLiveJoin Action = "live:Join"

	ActionWalletRead   Action = "wallet:Read"
	ActionOrderCreate  Action = "order:Create"
	ActionOrderListAll Action = "order:ListAll"
	ActionWalletCredit Action = "wallet:Credit"

	ActionUserReadSelf   Action = "user:ReadSelf"
	ActionUserUpdateSelf Action = "user:UpdateSelf"
	ActionUserList       Action = "user:List"
	ActionUserDisable    Action = "user:Disable"

	ActionIAMListRoles       Action = "iam:ListRoles"
	ActionIAMListAssignments Action = "iam:ListAssignments"
	ActionIAMAssignRole      Action = "iam:AssignRole"
	ActionIAMRevokeRole      Action = "iam:RevokeRole"

	ActionDashboardViewSelf  Action = "dashboard:ViewSelf"
	ActionDashboardViewAdmin Action = "dashboard:ViewAdmin"

	ActionAuditRead Action = "audit:Read"

	// Organization scope (Phase 5). Org roles are held per organization and are inherited
	// down to every class that belongs to that organization.
	ActionOrgCreate        Action = "org:Create"
	ActionOrgRead          Action = "org:Read"
	ActionOrgUpdate        Action = "org:Update"
	ActionOrgInviteMember  Action = "org:InviteMember"
	ActionOrgViewDashboard Action = "org:ViewDashboard"
)
