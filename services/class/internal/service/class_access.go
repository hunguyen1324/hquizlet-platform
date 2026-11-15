package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

type ClassAction struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

var ClassActions = []ClassAction{
	{"class-manga:Read", "Đọc truyện", "Truyện"}, {"class-manga:Create", "Nhập truyện", "Truyện"}, {"class-manga:Update", "Chỉnh sửa truyện và OCR", "Truyện"}, {"class-manga:Delete", "Xóa truyện", "Truyện"},
	{"class:Read", "Xem lớp", "Lớp học"}, {"class:Update", "Sửa thông tin và cài đặt", "Lớp học"}, {"class:Delete", "Xóa lớp", "Lớp học"}, {"class:ResetInvite", "Đổi mã mời", "Lớp học"},
	{"live:Host", "Tổ chức live quiz", "Lớp học"}, {"class-member:ViewProgress", "Xem tiến độ của thành viên", "Thành viên"},
	{"class:ViewDashboard", "Xem tổng quan", "Lớp học"}, {"class-access:Manage", "Quản lý role và gán quyền", "Phân quyền"},
	{"class-member:List", "Xem thành viên", "Thành viên"}, {"class-member:Add", "Thêm thành viên", "Thành viên"}, {"class-member:Remove", "Xóa thành viên", "Thành viên"}, {"class-member:UpdateRole", "Đổi vai trò thành viên", "Thành viên"},
	{"class-content:ReadStudySet", "Xem học phần", "Học phần"}, {"class-content:AssignStudySet", "Thêm học phần", "Học phần"}, {"class-content:UnassignStudySet", "Gỡ học phần", "Học phần"},
	{"class-content:ViewVideo", "Xem video", "Video"}, {"class-content:UploadVideo", "Thêm / tải video", "Video"}, {"class-video:Update", "Sửa video và chủ đề", "Video"}, {"class-video:Delete", "Xóa video", "Video"}, {"class-video:ManageAudience", "Quản lý quyền xem video", "Video"},
	{"class-document:Read", "Xem và tải tài liệu", "Tài liệu"}, {"class-document:Create", "Tải tài liệu lên", "Tài liệu"}, {"class-document:Update", "Đổi tên tài liệu", "Tài liệu"}, {"class-document:Delete", "Xóa tài liệu", "Tài liệu"},
	{"class-assignment:Read", "Xem bài tập đã giao", "Bài tập"}, {"class-assignment:Create", "Tạo bài tập", "Bài tập"}, {"class-assignment:Update", "Sửa bài tập và file", "Bài tập"}, {"class-assignment:Delete", "Xóa bài tập", "Bài tập"}, {"class-assignment:Publish", "Giao bài / thu hồi bài", "Bài tập"},
}

type ClassRole struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Managed     bool     `json:"managed"`
	Allow       []string `json:"allow"`
	Deny        []string `json:"deny"`
}

func allClassActions() []string {
	out := []string{}
	for _, a := range ClassActions {
		out = append(out, a.Key)
	}
	return out
}
func KnownClassAction(key string) bool {
	for _, a := range ClassActions {
		if a.Key == key {
			return true
		}
	}
	return false
}
func ManagedClassRoles() []ClassRole {
	read := []string{"class:Read", "class:ViewDashboard", "class-member:List", "class-content:ReadStudySet", "class-manga:Read", "class-content:ViewVideo", "class-document:Read", "class-assignment:Read"}
	teacher := []string{}
	for _, a := range ClassActions {
		if a.Key != "class:Delete" && a.Key != "class-access:Manage" {
			teacher = append(teacher, a.Key)
		}
	}
	assistant := append(append([]string{}, read...), "class-member:Add", "class-content:AssignStudySet", "class-content:UploadVideo", "class-document:Create", "class-assignment:Create", "class-assignment:Update", "class-assignment:Publish")
	content := append([]string{}, read...)
	for _, a := range ClassActions {
		if a.Group == "Truyện" || a.Group == "Video" || a.Group == "Tài liệu" || a.Group == "Học phần" {
			content = append(content, a.Key)
		}
	}
	return []ClassRole{
		{"owner", "Chủ lớp", "Toàn quyền; không thể gán hoặc thu hồi.", true, allClassActions(), []string{}},
		{"teacher", "Giáo viên", "Quản lý nội dung, bài tập, thành viên và thông tin lớp.", true, teacher, []string{}},
		{"assistant", "Trợ giảng", "Hỗ trợ thêm thành viên, nội dung và giao bài.", true, assistant, []string{}},
		{"content-manager", "Quản lý nội dung", "Quản lý video, tài liệu và học phần.", true, content, []string{}},
		{"manga-editor", "Chỉnh sửa truyện", "Đọc và chỉnh sửa truyện, chữ OCR trong lớp.", true, []string{"class:Read", "class-manga:Read", "class-manga:Update"}, []string{}},
		{"manga-deleter", "Xóa truyện", "Đọc và xóa truyện trong lớp.", true, []string{"class:Read", "class-manga:Read", "class-manga:Delete"}, []string{}},
		{"student", "Học viên", "Xem nội dung, tải tài liệu và xem bài tập.", true, read, []string{}},
		{"viewer", "Người xem", "Chỉ xem nội dung học tập, không xem danh sách thành viên.", true, []string{"class:Read", "class-content:ReadStudySet", "class-manga:Read", "class-content:ViewVideo", "class-document:Read", "class-assignment:Read"}, []string{}},
	}
}

// Explicit assignments replace the legacy teacher/student role. Deny wins across roles.
func EvaluateClassRoles(keys []string, roles []ClassRole) []string {
	allow, deny := map[string]bool{}, map[string]bool{}
	for _, key := range keys {
		for _, r := range roles {
			if r.Key != key || key == "owner" {
				continue
			}
			for _, a := range r.Allow {
				allow[a] = true
			}
			for _, a := range r.Deny {
				deny[a] = true
			}
		}
	}
	out := []string{}
	for a := range allow {
		if !deny[a] && KnownClassAction(a) {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

type ClassAccess struct {
	DB      *sql.DB
	Classes repository.ClassStore
	Members repository.MemberStore
}

func (a *ClassAccess) Roles(ctx context.Context, classID int64) ([]ClassRole, error) {
	out := ManagedClassRoles()
	rows, err := a.DB.QueryContext(ctx, `SELECT key,name,description,allow_actions,deny_actions FROM class_role_definitions WHERE class_id=$1 ORDER BY name,key`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ClassRole
		var allow, deny []byte
		if err = rows.Scan(&r.Key, &r.Name, &r.Description, &allow, &deny); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(allow, &r.Allow); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(deny, &r.Deny); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (a *ClassAccess) Effective(ctx context.Context, classID, userID int64) ([]string, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}
	cls, err := a.Classes.GetByID(ctx, classID)
	if err != nil {
		return nil, err
	}
	if cls.OwnerUserID == userID {
		return allClassActions(), nil
	}
	role, err := a.Members.GetRole(ctx, classID, userID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	keys := []string{role}
	var raw []byte
	err = a.DB.QueryRowContext(ctx, `SELECT role_keys FROM class_member_access WHERE class_id=$1 AND user_id=$2`, classID, userID).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	roles := ManagedClassRoles()
	if err == nil {
		if err = json.Unmarshal(raw, &keys); err != nil {
			return nil, err
		}
		roles, err = a.Roles(ctx, classID)
		if err != nil {
			return nil, err
		}
	}
	return EvaluateClassRoles(keys, roles), nil
}
func (a *ClassAccess) Require(ctx context.Context, classID, userID int64, action string) error {
	actions, err := a.Effective(ctx, classID, userID)
	if err != nil {
		return err
	}
	for _, x := range actions {
		if x == action {
			return nil
		}
	}
	return ErrForbidden
}

func (s *ClassDashboardService) WithClassAccess(a *ClassAccess) *ClassDashboardService {
	s.access = a
	return s
}

func ConfigureClassAccess(db *sql.DB, classes *ClassService, members *MemberService, sets *ClassStudySetService) *ClassAccess {
	a := &ClassAccess{DB: db, Classes: classes.classes, Members: classes.members}
	classes.access = a
	members.access = a
	sets.access = a
	return a
}

// Membership administrators cannot use the legacy teacher/student endpoint to grant
// actions they do not hold themselves or bypass an explicit deny on their own role.
func (a *ClassAccess) CanDelegateLegacyRole(ctx context.Context, classID, userID int64, role string) error {
	current, err := a.Effective(ctx, classID, userID)
	if err != nil {
		return err
	}
	held := map[string]bool{}
	for _, key := range current {
		held[key] = true
	}
	for _, key := range EvaluateClassRoles([]string{role}, ManagedClassRoles()) {
		if !held[key] {
			return ErrForbidden
		}
	}
	return nil
}
