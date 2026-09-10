package authz

import (
	"fmt"
	"sync"

	"go-web/model"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
)

type Permission struct {
	Resource string
	Action   string
}

var (
	UserRead      = Permission{Resource: "user", Action: "read"}
	UserWrite     = Permission{Resource: "user", Action: "write"}
	CanvasRead    = Permission{Resource: "canvas", Action: "read"}
	CanvasWrite   = Permission{Resource: "canvas", Action: "write"}
	CanvasAI      = Permission{Resource: "canvas", Action: "ai"}
	CanvasAudit   = Permission{Resource: "canvas", Action: "audit"}
	MaterialRead  = Permission{Resource: "material", Action: "read"}
	MaterialWrite = Permission{Resource: "material", Action: "write"}
)

const (
	RoleUserKey    = "user"
	RoleVIP1Key    = "vip1"
	RoleVIP2Key    = "vip2"
	RoleAuditorKey = "auditor"
	RoleAdminKey   = "admin"
)

const modelText = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

var (
	enforcerMu sync.RWMutex
	enforcer   *casbin.Enforcer
)

func RoleSubject(roleKey string) string {
	return "role:" + roleKey
}

func SubjectFromUser(role, vipLevel int) string {
	switch {
	case role >= model.RoleAdmin:
		return RoleSubject(RoleAdminKey)
	case role >= model.RoleAuditor:
		return RoleSubject(RoleAuditorKey)
	case vipLevel >= model.Vip2:
		return RoleSubject(RoleVIP2Key)
	case vipLevel >= model.Vip1:
		return RoleSubject(RoleVIP1Key)
	default:
		return RoleSubject(RoleUserKey)
	}
}

func Init() error {
	m, err := casbinmodel.NewModelFromString(modelText)
	if err != nil {
		return err
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return err
	}
	if err := seedPolicies(e); err != nil {
		return err
	}

	enforcerMu.Lock()
	enforcer = e
	enforcerMu.Unlock()
	return nil
}

func seedPolicies(e *casbin.Enforcer) error {
	policies := [][]string{
		{RoleSubject(RoleUserKey), CanvasRead.Resource, CanvasRead.Action},
		{RoleSubject(RoleUserKey), CanvasWrite.Resource, CanvasWrite.Action},
		{RoleSubject(RoleUserKey), MaterialRead.Resource, MaterialRead.Action},
		{RoleSubject(RoleUserKey), UserRead.Resource, UserRead.Action},
		{RoleSubject(RoleVIP1Key), CanvasAI.Resource, CanvasAI.Action},
		{RoleSubject(RoleVIP2Key), MaterialWrite.Resource, MaterialWrite.Action},
		{RoleSubject(RoleAuditorKey), CanvasAudit.Resource, CanvasAudit.Action},
		{RoleSubject(RoleAdminKey), UserWrite.Resource, UserWrite.Action},
	}
	for _, p := range policies {
		if _, err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return fmt.Errorf("add policy %v: %w", p, err)
		}
	}

	groupings := [][]string{
		{RoleSubject(RoleVIP1Key), RoleSubject(RoleUserKey)},
		{RoleSubject(RoleVIP2Key), RoleSubject(RoleVIP1Key)},
		{RoleSubject(RoleAuditorKey), RoleSubject(RoleUserKey)},
		{RoleSubject(RoleAdminKey), RoleSubject(RoleVIP2Key)},
		{RoleSubject(RoleAdminKey), RoleSubject(RoleAuditorKey)},
	}
	for _, g := range groupings {
		if _, err := e.AddGroupingPolicy(g[0], g[1]); err != nil {
			return err
		}
	}
	return nil
}

func Can(role, vipLevel int, permission Permission) bool {
	enforcerMu.RLock()
	e := enforcer
	enforcerMu.RUnlock()
	if e == nil {
		return false
	}
	ok, err := e.Enforce(SubjectFromUser(role, vipLevel), permission.Resource, permission.Action)
	if err != nil {
		return false
	}
	return ok
}

type Capabilities struct {
	CanEdit         bool `json:"can_edit"`
	CanUpdate       bool `json:"can_update"`
	CanUseAI        bool `json:"can_use_ai"`
	CanSaveMaterial bool `json:"can_save_material"`
	CanManageUser   bool `json:"can_manage_user"`
	CanAuditCanvas  bool `json:"can_audit_canvas"`
}

func Caps(role, vipLevel int) Capabilities {
	return Capabilities{
		CanEdit:         Can(role, vipLevel, CanvasWrite),
		CanUpdate:       Can(role, vipLevel, CanvasWrite),
		CanUseAI:        Can(role, vipLevel, CanvasAI),
		CanSaveMaterial: Can(role, vipLevel, MaterialWrite),
		CanManageUser:   Can(role, vipLevel, UserWrite),
		CanAuditCanvas:  Can(role, vipLevel, CanvasAudit),
	}
}
