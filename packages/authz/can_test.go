package authz

import "testing"

func TestManagedRolesComposeWithPlatformAdmin(t *testing.T) {
	admin := Principal{UserID: 1, PlatformRoles: []string{RolePlatformAdmin, RoleTeacher, RoleStudent}, ClassRole: "student", ClassID: 10}
	for _, action := range []Action{ActionStudySetUpdate, ActionStudySetDelete, ActionStudySetSetPrice} {
		if !Can(admin, action, Resource{Type: "study-set", ID: 2, OwnerUserID: 99}) {
			t.Fatalf("admin lost %s through another managed role", action)
		}
	}
	if !Can(admin, ActionClassMemberViewProgress, Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 99}) {
		t.Fatal("admin lost class-wide progress through membership")
	}
	teacher := Principal{UserID: 1, PlatformRoles: []string{RoleTeacher, RoleStudent}}
	if !Can(teacher, ActionStudySetSetPrice, Resource{Type: "study-set", ID: 2, OwnerUserID: 1}) {
		t.Fatal("Teacher grant cannot compose with Student")
	}
}

func TestCanDefaultDeny(t *testing.T) {
	p := Principal{UserID: 1, PlatformRoles: []string{RoleStudent}}
	if Can(p, ActionUserList, Resource{Type: "user", ID: 2}) {
		t.Fatal("expected default deny for admin user list")
	}
}

func TestDenyBeatsAllow(t *testing.T) {
	p := Principal{UserID: 1, PlatformRoles: []string{RoleStudent}}
	res := Resource{Type: "study-set", ID: 99, OwnerUserID: 2}
	if Can(p, ActionStudySetUpdate, res) {
		t.Fatal("student must not update another user's study set")
	}
}

func TestStudentOwnStudySet(t *testing.T) {
	p := Principal{UserID: 5, PlatformRoles: []string{RoleStudent}}
	res := Resource{Type: "study-set", ID: 10, OwnerUserID: 5}
	if !Can(p, ActionStudySetUpdate, res) {
		t.Fatal("student should update own study set")
	}
	if Can(p, ActionStudySetSetPrice, res) {
		t.Fatal("student must not set price")
	}
}

func TestDefaultDenyForUnknownAction(t *testing.T) {
	p := Principal{UserID: 1, PlatformRoles: []string{RoleTeacher}}
	if Can(p, Action("class:Teleport"), Resource{Type: "class", ID: 10, ClassID: 10}) {
		t.Fatal("unknown actions must be denied by default")
	}
}

func TestTeacherSetPriceOwnOnly(t *testing.T) {
	p := Principal{UserID: 3, PlatformRoles: []string{RoleTeacher}}
	if !Can(p, ActionStudySetSetPrice, Resource{Type: "study-set", ID: 1, OwnerUserID: 3}) {
		t.Fatal("teacher should set price on own set")
	}
	if Can(p, ActionStudySetSetPrice, Resource{Type: "study-set", ID: 2, OwnerUserID: 9}) {
		t.Fatal("teacher must not set price on others' sets")
	}
}

func TestPlatformAdmin(t *testing.T) {
	p := Principal{UserID: 1, LegacyRole: "admin"}
	if !Can(p, ActionDashboardViewAdmin, Resource{Type: "platform"}) {
		t.Fatal("legacy admin should access admin dashboard")
	}
	if !Can(p, ActionUserDisable, Resource{Type: "user", ID: 99}) {
		t.Fatal("platform admin should disable users")
	}
}

func TestClassScopeInheritance(t *testing.T) {
	owner := Principal{UserID: 1, PlatformRoles: []string{RoleTeacher}, ClassRole: "owner", ClassID: 10}
	res := Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 1}
	if !Can(owner, ActionClassDelete, res) {
		t.Fatal("class owner should delete class")
	}

	teacher := Principal{UserID: 2, ClassRole: "teacher", ClassID: 10}
	if Can(teacher, ActionClassDelete, res) {
		t.Fatal("class teacher must not delete class")
	}
	if !Can(teacher, ActionClassContentAssign, res) {
		t.Fatal("class teacher should assign content")
	}
}

func TestClassStudentProgressIsolation(t *testing.T) {
	studentA := Principal{UserID: 7, ClassRole: "student", ClassID: 10}
	classRes := Resource{Type: "class", ID: 10, ClassID: 10}
	self := Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 7}
	other := Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 8}

	if !Can(studentA, ActionClassMemberViewProgress, self) {
		t.Fatal("student should view own progress")
	}
	if Can(studentA, ActionClassMemberViewProgress, other) {
		t.Fatal("student must not view peer progress")
	}
	if !Can(studentA, ActionClassViewDashboard, classRes) {
		t.Fatal("student should view class dashboard")
	}
}

func TestTeacherCreateClass(t *testing.T) {
	p := Principal{UserID: 1, PlatformRoles: []string{RoleTeacher}}
	if !Can(p, ActionClassCreate, Resource{Type: "class"}) {
		t.Fatal("teacher should create class")
	}
	student := Principal{UserID: 2, PlatformRoles: []string{RoleStudent}}
	if Can(student, ActionClassCreate, Resource{Type: "class"}) {
		t.Fatal("student must not create class")
	}
}

func TestLegacyAdminHeaderCompat(t *testing.T) {
	p := Principal{UserID: 1, LegacyRole: "admin", PlatformRoles: nil}
	if !Can(p, ActionOrderListAll, Resource{Type: "order"}) {
		t.Fatal("legacy admin without platform roles header should still work")
	}
}

// TestPermissionMatrix walks every row of the role x action matrix in
// docs/dashboard-iam-plan.md §7 so a policy change cannot silently widen access.
func TestPermissionMatrix(t *testing.T) {
	studySet := func(owner int64) Resource {
		return Resource{Type: "study-set", ID: 100, OwnerUserID: owner}
	}
	class := func(owner int64) Resource {
		return Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: owner}
	}

	student := Principal{UserID: 3, PlatformRoles: []string{RoleStudent}}
	teacher := Principal{UserID: 2, PlatformRoles: []string{RoleTeacher}}
	classStudent := Principal{UserID: 30, ClassRole: "student", ClassID: 10}
	classTeacher := Principal{UserID: 31, ClassRole: "teacher", ClassID: 10}
	classOwner := Principal{UserID: 32, ClassRole: "owner", ClassID: 10}
	admin := Principal{UserID: 1, PlatformRoles: []string{RolePlatformAdmin}}

	tests := []struct {
		name   string
		p      Principal
		action Action
		res    Resource
		want   bool
	}{
		// Study sets: own vs other people's
		{"student creates own study set", student, ActionStudySetCreate, Resource{Type: "study-set", OwnerUserID: 3}, true},
		{"student edits own study set", student, ActionStudySetUpdate, studySet(3), true},
		{"student deletes own study set", student, ActionStudySetDelete, studySet(3), true},
		{"teacher edits own study set", teacher, ActionStudySetUpdate, studySet(2), true},
		{"student edits another study set", student, ActionStudySetUpdate, studySet(99), false},
		{"teacher deletes another study set", teacher, ActionStudySetDelete, studySet(99), false},
		{"admin edits another study set", admin, ActionStudySetUpdate, studySet(99), true},
		{"student cannot price a study set", student, ActionStudySetSetPrice, studySet(3), false},
		{"teacher prices own study set", teacher, ActionStudySetSetPrice, studySet(2), true},

		// Classes
		{"teacher creates a class", teacher, ActionClassCreate, Resource{Type: "class"}, true},
		{"student cannot create a class", student, ActionClassCreate, Resource{Type: "class"}, false},
		{"student joins a class by code", student, ActionClassJoin, Resource{Type: "class"}, true},
		{"teacher joins a class by code", teacher, ActionClassJoin, Resource{Type: "class"}, true},
		{"class owner deletes class", classOwner, ActionClassDelete, class(32), true},
		{"class teacher cannot delete class", classTeacher, ActionClassDelete, class(32), false},
		{"class student cannot delete class", classStudent, ActionClassDelete, class(32), false},
		{"class owner resets invite code", classOwner, ActionClassResetInvite, class(32), true},
		{"class teacher cannot reset invite code", classTeacher, ActionClassResetInvite, class(32), false},
		{"class role does not leak across classes", classStudent, ActionClassRead, Resource{Type: "class", ID: 11, ClassID: 11}, false},

		// Class content
		{"class student views assigned content", classStudent, ActionClassContentView, class(32), true},
		{"class teacher assigns content", classTeacher, ActionClassContentAssign, class(32), true},
		{"class owner assigns content", classOwner, ActionClassContentAssign, class(32), true},
		{"class student cannot assign content", classStudent, ActionClassContentAssign, class(32), false},

		// Class members and progress
		{"class teacher lists members", classTeacher, ActionClassMemberList, class(32), true},
		{"class owner lists members", classOwner, ActionClassMemberList, class(32), true},
		{"class student can view the roster", classStudent, ActionClassMemberList, class(32), true},
		{"class teacher adds a member", classTeacher, ActionClassMemberAdd, class(32), true},
		{"class teacher removes a member", classTeacher, ActionClassMemberRemove, class(32), true},
		{"class teacher updates a member role", classTeacher, ActionClassMemberUpdateRole, class(32), true},
		{"class student cannot update member roles", classStudent, ActionClassMemberUpdateRole, class(32), false},
		{"class student views own progress", classStudent, ActionClassMemberViewProgress, class(30), true},
		{"class student cannot view peer progress", classStudent, ActionClassMemberViewProgress, class(31), false},
		{"class teacher views class-wide progress", classTeacher, ActionClassMemberViewProgress, class(30), true},
		{"class owner views class-wide progress", classOwner, ActionClassMemberViewProgress, class(30), true},

		// Live quiz
		{"teacher hosts a live quiz", teacher, ActionLiveHost, Resource{Type: "live"}, true},
		{"class teacher hosts a live quiz", classTeacher, ActionLiveHost, class(32), true},
		{"class owner hosts a live quiz", classOwner, ActionLiveHost, class(32), true},
		{"student cannot host a live quiz", student, ActionLiveHost, Resource{Type: "live"}, false},
		{"class student cannot host a live quiz", classStudent, ActionLiveHost, class(32), false},

		// Admin console and IAM
		{"platform admin views admin dashboard", admin, ActionDashboardViewAdmin, Resource{Type: "platform"}, true},
		{"student cannot view admin dashboard", student, ActionDashboardViewAdmin, Resource{Type: "platform"}, false},
		{"teacher cannot view admin dashboard", teacher, ActionDashboardViewAdmin, Resource{Type: "platform"}, false},
		{"class owner cannot view admin dashboard", classOwner, ActionDashboardViewAdmin, Resource{Type: "platform"}, false},
		{"platform admin assigns roles", admin, ActionIAMAssignRole, Resource{Type: "user", ID: 9}, true},
		{"platform admin revokes roles", admin, ActionIAMRevokeRole, Resource{Type: "user", ID: 9}, true},
		{"student cannot assign roles", student, ActionIAMAssignRole, Resource{Type: "user", ID: 3}, false},
		{"teacher cannot assign roles", teacher, ActionIAMAssignRole, Resource{Type: "user", ID: 2}, false},
		{"platform admin disables users", admin, ActionUserDisable, Resource{Type: "user", ID: 9}, true},
		{"teacher cannot disable users", teacher, ActionUserDisable, Resource{Type: "user", ID: 9}, false},
		{"platform admin reads audit log", admin, ActionAuditRead, Resource{Type: "audit"}, true},
		{"student cannot read audit log", student, ActionAuditRead, Resource{Type: "audit"}, false},

		// Money
		{"platform admin credits a wallet", admin, ActionWalletCredit, Resource{Type: "wallet"}, true},
		{"platform admin lists all orders", admin, ActionOrderListAll, Resource{Type: "order"}, true},
		{"teacher cannot credit a wallet", teacher, ActionWalletCredit, Resource{Type: "wallet"}, false},
		{"student cannot list all orders", student, ActionOrderListAll, Resource{Type: "order"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Can(tt.p, tt.action, tt.res); got != tt.want {
				t.Fatalf("Can(%s, %s, %s) = %v, want %v", tt.p.RoleLabel(), tt.action, tt.res.ARN(), got, tt.want)
			}
		})
	}
}

// TestOrgScopeInheritance covers the Phase 5 rule set: org roles apply to the organization
// itself and are inherited by every class whose org_id is that organization, while never
// leaking to another organization, a personal class, or the platform scope.
func TestOrgScopeInheritance(t *testing.T) {
	orgClass := func(orgID int64) Resource {
		return Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 99, OrgID: orgID}
	}
	orgAdmin := Principal{UserID: 1, OrgRoles: map[int64][]string{7: {RoleOrgAdmin}}}
	orgTeacher := Principal{UserID: 2, OrgRoles: map[int64][]string{7: {RoleOrgTeacher}}}
	orgStudent := Principal{UserID: 3, OrgRoles: map[int64][]string{7: {RoleOrgStudent}}}

	tests := []struct {
		name   string
		p      Principal
		action Action
		res    Resource
		want   bool
	}{
		// OrgAdmin: full control over the org and its classes.
		{"org admin reads the org", orgAdmin, ActionOrgRead, Resource{Type: "org", ID: 7, OrgID: 7}, true},
		{"org admin views org dashboard", orgAdmin, ActionOrgViewDashboard, Resource{Type: "org", ID: 7, OrgID: 7}, true},
		{"org admin invites members", orgAdmin, ActionOrgInviteMember, Resource{Type: "org", ID: 7, OrgID: 7}, true},
		{"org admin deletes a class of the org", orgAdmin, ActionClassDelete, orgClass(7), true},
		{"org admin resets a class invite code", orgAdmin, ActionClassResetInvite, orgClass(7), true},
		{"org admin updates member roles", orgAdmin, ActionClassMemberUpdateRole, orgClass(7), true},
		{"org admin inherits onto class progress", orgAdmin, ActionClassMemberViewProgress, orgClass(7), true},

		// OrgTeacher: teaches, never destroys.
		{"org teacher views class dashboard", orgTeacher, ActionClassViewDashboard, orgClass(7), true},
		{"org teacher assigns content", orgTeacher, ActionClassContentAssign, orgClass(7), true},
		{"org teacher views class-wide progress", orgTeacher, ActionClassMemberViewProgress, orgClass(7), true},
		{"org teacher cannot delete a class", orgTeacher, ActionClassDelete, orgClass(7), false},
		{"org teacher cannot reset invite code", orgTeacher, ActionClassResetInvite, orgClass(7), false},
		{"org teacher cannot manage the org itself", orgTeacher, ActionOrgInviteMember, Resource{Type: "org", ID: 7, OrgID: 7}, false},

		// OrgStudent: learns, sees only own progress.
		{"org student views assigned content", orgStudent, ActionClassContentView, orgClass(7), true},
		{"org student views own progress", orgStudent, ActionClassMemberViewProgress, Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 3, OrgID: 7}, true},
		{"org student cannot view peer progress", orgStudent, ActionClassMemberViewProgress, Resource{Type: "class", ID: 10, ClassID: 10, OwnerUserID: 4, OrgID: 7}, false},
		{"org student cannot assign content", orgStudent, ActionClassContentAssign, orgClass(7), false},

		// Isolation: another org, a personal class, and the platform scope get nothing.
		{"org admin of org 7 has nothing in org 9", orgAdmin, ActionClassDelete, orgClass(9), false},
		{"org teacher of org 7 has nothing in org 9", orgTeacher, ActionClassViewDashboard, orgClass(9), false},
		{"org roles do not apply to personal classes", orgAdmin, ActionClassDelete, orgClass(0), false},
		{"org admin is not a platform admin", orgAdmin, ActionDashboardViewAdmin, Resource{Type: "platform"}, false},
		{"org admin cannot assign platform roles", orgAdmin, ActionIAMAssignRole, Resource{Type: "user", ID: 5}, false},
		{"org admin needs the org to exist for org actions", orgAdmin, ActionOrgViewDashboard, Resource{Type: "org", ID: 7}, false},
		{"no org roles means no org access", Principal{UserID: 8, PlatformRoles: []string{RoleStudent}}, ActionOrgViewDashboard, Resource{Type: "org", ID: 7, OrgID: 7}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Can(tt.p, tt.action, tt.res); got != tt.want {
				t.Fatalf("Can(%v, %s, %s) = %v, want %v", tt.p.OrgRoles, tt.action, tt.res.ARN(), got, tt.want)
			}
		})
	}
}

// TestPlatformAdminReachesOrgScope keeps the built-in admin able to manage organizations.
func TestPlatformAdminReachesOrgScope(t *testing.T) {
	admin := Principal{UserID: 1, PlatformRoles: []string{RolePlatformAdmin}}
	res := Resource{Type: "org", ID: 3, OrgID: 3}
	for _, action := range []Action{ActionOrgCreate, ActionOrgRead, ActionOrgUpdate, ActionOrgInviteMember, ActionOrgViewDashboard} {
		if !Can(admin, action, res) {
			t.Fatalf("platform admin should perform %s", action)
		}
	}
	if !Can(admin, ActionOrgCreate, Resource{Type: "org"}) {
		t.Fatal("platform admin should create organizations")
	}
}

func TestParseOrgRoles(t *testing.T) {
	cases := []struct {
		name   string
		header string
		orgID  int64
		role   string
		want   bool
	}{
		{"single entry", "7:OrgAdmin", 7, RoleOrgAdmin, true},
		{"spaces are tolerated", " 7 : OrgTeacher , 9:OrgStudent ", 9, RoleOrgStudent, true},
		{"unknown org", "7:OrgAdmin", 8, RoleOrgAdmin, false},
		{"unknown role in known org", "7:OrgAdmin", 7, RoleOrgTeacher, false},
		{"garbage entry is skipped", "nonsense,7:OrgAdmin", 7, RoleOrgAdmin, true},
		{"negative org id is skipped", "-3:OrgAdmin,5:OrgStudent", 5, RoleOrgStudent, true},
		{"missing role", "7:", 7, RoleOrgAdmin, false},
		{"missing org id", ":OrgAdmin", 7, RoleOrgAdmin, false},
		{"empty header", "", 7, RoleOrgAdmin, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roles := ParseOrgRoles(tc.header)
			got := false
			for _, r := range roles[tc.orgID] {
				if r == tc.role {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("ParseOrgRoles(%q)[%d] contains %s = %v, want %v", tc.header, tc.orgID, tc.role, got, tc.want)
			}
		})
	}

	if entry := FormatOrgRole(12, RoleOrgTeacher); entry != "12:OrgTeacher" {
		t.Fatalf("FormatOrgRole = %q", entry)
	}
	if roles := ParseOrgRoles(FormatOrgRole(12, RoleOrgTeacher)); !contains(roles[12], RoleOrgTeacher) {
		t.Fatal("FormatOrgRole/ParseOrgRoles must round-trip")
	}
}

// RoleLabel is a small test helper to make failure output readable.
func (p Principal) RoleLabel() string {
	if p.ClassRole != "" {
		return "class:" + p.ClassRole
	}
	if len(p.PlatformRoles) > 0 {
		return p.PlatformRoles[0]
	}
	return "anonymous"
}
