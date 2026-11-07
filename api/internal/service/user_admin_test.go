package service

import (
	"errors"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func TestMayManageFollowsD02(t *testing.T) {
	cases := []struct {
		actor  model.Role
		target model.Role
		want   bool
	}{
		{model.RoleAdmin, model.RoleStudent, true},
		{model.RoleAdmin, model.RoleOperator, true},
		{model.RoleAdmin, model.RoleAdmin, true},
		{model.RoleAdmin, model.RolePlatformAdmin, false},
		{model.RoleOperator, model.RoleStudent, true},
		{model.RoleOperator, model.RoleOperator, false},
		{model.RoleOperator, model.RoleAdmin, false},
		{model.RoleStudent, model.RoleStudent, false},
	}
	for _, c := range cases {
		err := mayManage(model.Principal{Role: c.actor}, c.target)
		if (err == nil) != c.want || (err != nil && !errors.Is(err, ErrUserForbidden)) {
			t.Errorf("%s managing %s: err = %v, want allowed = %v", c.actor, c.target, err, c.want)
		}
	}
}

func TestListRolesNarrowsAnOperatorToStudents(t *testing.T) {
	op, admin := model.Principal{Role: model.RoleOperator}, model.Principal{Role: model.RoleAdmin}
	if roles, err := listRoles(op, ""); err != nil || len(roles) != 1 || roles[0] != model.RoleStudent {
		t.Errorf("operator default = %v, %v", roles, err)
	}
	for _, role := range []string{"operator", "admin", "staff"} {
		if _, err := listRoles(op, role); !errors.Is(err, ErrUserForbidden) {
			t.Errorf("operator listing %s: err = %v, want forbidden", role, err)
		}
	}
	if roles, err := listRoles(admin, "staff"); err != nil || len(roles) != 2 {
		t.Errorf("admin staff = %v, %v", roles, err)
	}
	var input *InputError
	if _, err := listRoles(admin, "platform_admin"); !errors.As(err, &input) {
		t.Errorf("unknown role: err = %v, want an input error", err)
	}
}
