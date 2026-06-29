// Package authz wires Casbin RBAC into the app: it owns the access-control
// model, the default policy matrix, and a thin enforcer constructor backed by
// the application's Postgres database.
//
// Division of labour: Casbin answers "may this ROLE perform ACTION on
// RESOURCE?". It does NOT answer "does this user own this row?" — ownership
// stays in the service layer, which already scopes financings/payments by
// userID. Keeping the two concerns separate avoids writing a Casbin policy
// line per object.
package authz

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// Resources — the objects an action targets. Keep these in sync with the route
// groups so each handler maps to exactly one resource.
const (
	ResourceProfile    = "profile"    // the caller's own account (/api/me)
	ResourceUsers      = "users"      // user administration (/api/users)
	ResourceFinancings = "financings" // financing contracts (/api/financings)
	ResourcePayments   = "payments"   // payments (/api/payments)
)

// Actions — the verbs. "*" in a policy grants every action on a resource.
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionSign    = "sign"    // sign an akad (financing-specific)
	ActionPay     = "pay"     // pay an installment (financing-specific)
	ActionApprove = "approve" // set terms & approve a financing application (back office)
	ActionAny     = "*"
)

// rbacModel is an RBAC model with role inheritance and resource/action
// wildcards. g(r.sub, p.sub) lets a request subject match a policy subject
// either directly or through a role it inherits (e.g. admin -> user).
const rbacModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (r.obj == p.obj || p.obj == "*") && (r.act == p.act || p.act == "*")
`

// defaultPolicies is the permission matrix.
//
//   - user: self-service plus financing/payment actions on their own records.
//     Users APPLY for financings but do not set their own margin or approve them.
//   - staff: read-only oversight across users/financings/payments (back office),
//     plus underwriting — they APPROVE applications (set terms). Still NO
//     master-data writes (no users create/update/delete) and no signing/paying
//     on a customer's behalf.
//   - admin: full user administration and, via the admin->user grouping below,
//     everything the user role can do; plus approve (granted explicitly, since
//     admin inherits user — not staff).
var defaultPolicies = [][]string{
	{"user", ResourceProfile, ActionRead},
	{"user", ResourceProfile, ActionUpdate},
	{"user", ResourceFinancings, ActionCreate},
	{"user", ResourceFinancings, ActionRead},
	{"user", ResourceFinancings, ActionSign},
	{"user", ResourceFinancings, ActionPay},
	{"user", ResourcePayments, ActionCreate},
	{"user", ResourcePayments, ActionRead},

	{"staff", ResourceProfile, ActionRead},
	{"staff", ResourceProfile, ActionUpdate},
	{"staff", ResourceUsers, ActionRead},
	{"staff", ResourceFinancings, ActionRead},
	{"staff", ResourceFinancings, ActionApprove},
	{"staff", ResourcePayments, ActionRead},

	{"admin", ResourceUsers, ActionAny},
	{"admin", ResourceFinancings, ActionApprove},
}

// defaultGroupings establishes role inheritance: admin is also a user.
var defaultGroupings = [][]string{
	{"admin", "user"},
}

// NewEnforcer builds a Casbin enforcer backed by the given GORM database. On a
// fresh database it seeds the default policy matrix; on subsequent boots it
// loads whatever is persisted (so runtime policy edits survive restarts).
func NewEnforcer(db *gorm.DB) (*casbin.Enforcer, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("casbin adapter: %w", err)
	}

	m, err := model.NewModelFromString(rbacModel)
	if err != nil {
		return nil, fmt.Errorf("casbin model: %w", err)
	}

	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}

	if err := seed(enforcer); err != nil {
		return nil, err
	}
	return enforcer, nil
}

// seed makes sure every default policy and grouping line exists, adding any that
// are missing. It is additive and idempotent: AddPolicy/AddGroupingPolicy no-op
// on a line that is already present, so re-running is safe, and lines an admin
// added at runtime are left untouched.
//
// Backfilling per line (rather than only seeding an empty table) is what lets a
// NEW default — e.g. the financings/approve grants — reach an already-seeded
// database on the next boot, with no manual SQL.
//
// Trade-off: a default line an admin deliberately deleted at runtime is restored
// on the next boot. Defaults are the baseline the app needs to function, so
// restoring them is intentional — customise by editing defaultPolicies, not by
// deleting rows.
func seed(e *casbin.Enforcer) error {
	for _, p := range defaultPolicies {
		if _, err := e.AddPolicy(p); err != nil {
			return fmt.Errorf("seed policy %v: %w", p, err)
		}
	}
	for _, g := range defaultGroupings {
		if _, err := e.AddGroupingPolicy(g); err != nil {
			return fmt.Errorf("seed grouping %v: %w", g, err)
		}
	}
	return nil
}
