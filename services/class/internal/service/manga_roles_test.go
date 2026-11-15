package service

import (
	"slices"
	"testing"
)

func TestMangaRolesKeepEditingAndDeletionSeparate(t *testing.T) {
	roles := ManagedClassRoles()
	for _, tc := range []struct{ role, allow, deny string }{
		{"manga-editor", "class-manga:Update", "class-manga:Delete"},
		{"manga-deleter", "class-manga:Delete", "class-manga:Update"},
		{"student", "class-manga:Read", "class-manga:Update"},
	} {
		actions := EvaluateClassRoles([]string{tc.role}, roles)
		if !slices.Contains(actions, tc.allow) || slices.Contains(actions, tc.deny) {
			t.Fatalf("%s: %v", tc.role, actions)
		}
	}
}
