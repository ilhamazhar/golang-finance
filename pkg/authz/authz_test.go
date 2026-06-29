package authz

import (
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// testEnforcer builds an in-memory enforcer from the production model and the
// production seed data, with no database adapter. It exercises the exact
// rbacModel and defaultPolicies/defaultGroupings that NewEnforcer ships.
func testEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(rbacModel)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("build enforcer: %v", err)
	}
	if err := seed(e); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return e
}

func TestPolicyMatrix(t *testing.T) {
	e := testEnforcer(t)

	cases := []struct {
		name          string
		sub, obj, act string
		want          bool
	}{
		// --- user: self-service + financing/payment ---
		{"user reads own profile", "user", ResourceProfile, ActionRead, true},
		{"user updates own profile", "user", ResourceProfile, ActionUpdate, true},
		{"user creates financing", "user", ResourceFinancings, ActionCreate, true},
		{"user reads financing", "user", ResourceFinancings, ActionRead, true},
		{"user signs akad", "user", ResourceFinancings, ActionSign, true},
		{"user pays installment", "user", ResourceFinancings, ActionPay, true},
		{"user creates payment", "user", ResourcePayments, ActionCreate, true},
		{"user reads payment", "user", ResourcePayments, ActionRead, true},

		// --- user: cannot approve their own application (margin is the institution's) ---
		{"user cannot approve financing", "user", ResourceFinancings, ActionApprove, false},

		// --- user: denied all user administration ---
		{"user cannot list users", "user", ResourceUsers, ActionRead, false},
		{"user cannot update users", "user", ResourceUsers, ActionUpdate, false},
		{"user cannot delete users", "user", ResourceUsers, ActionDelete, false},

		// --- admin: full user administration via wildcard ---
		{"admin reads users", "admin", ResourceUsers, ActionRead, true},
		{"admin updates users", "admin", ResourceUsers, ActionUpdate, true},
		{"admin deletes users", "admin", ResourceUsers, ActionDelete, true},
		{"admin creates users", "admin", ResourceUsers, ActionCreate, true},

		// --- admin: inherits every user permission (g: admin -> user) ---
		{"admin inherits financing create", "admin", ResourceFinancings, ActionCreate, true},
		{"admin inherits profile read", "admin", ResourceProfile, ActionRead, true},
		{"admin inherits payment read", "admin", ResourcePayments, ActionRead, true},
		{"admin approves financing", "admin", ResourceFinancings, ActionApprove, true},

		// --- staff: read-only oversight + own profile + underwriting (approve) ---
		{"staff reads users", "staff", ResourceUsers, ActionRead, true},
		{"staff reads financings", "staff", ResourceFinancings, ActionRead, true},
		{"staff approves financing", "staff", ResourceFinancings, ActionApprove, true},
		{"staff reads payments", "staff", ResourcePayments, ActionRead, true},
		{"staff reads own profile", "staff", ResourceProfile, ActionRead, true},
		{"staff updates own profile", "staff", ResourceProfile, ActionUpdate, true},

		// --- staff: denied master-data writes and the customer's own actions ---
		{"staff cannot create users", "staff", ResourceUsers, ActionCreate, false},
		{"staff cannot update users", "staff", ResourceUsers, ActionUpdate, false},
		{"staff cannot delete users", "staff", ResourceUsers, ActionDelete, false},
		{"staff cannot create financing", "staff", ResourceFinancings, ActionCreate, false},
		{"staff cannot sign akad", "staff", ResourceFinancings, ActionSign, false},
		{"staff cannot pay installment", "staff", ResourceFinancings, ActionPay, false},
		{"staff cannot create payment", "staff", ResourcePayments, ActionCreate, false},

		// --- negative: unknown subjects and actions are denied ---
		{"unknown role denied", "guest", ResourceProfile, ActionRead, false},
		{"unknown action denied", "user", ResourceFinancings, "destroy", false},
		{"unknown resource denied", "user", "reports", ActionRead, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := e.Enforce(tc.sub, tc.obj, tc.act)
			if err != nil {
				t.Fatalf("enforce(%q, %q, %q): %v", tc.sub, tc.obj, tc.act, err)
			}
			if got != tc.want {
				t.Errorf("enforce(%q, %q, %q) = %v, want %v", tc.sub, tc.obj, tc.act, got, tc.want)
			}
		})
	}
}

// TestAdminInheritsUser asserts the role hierarchy directly: every permission
// granted to user must also be granted to admin through the g grouping.
func TestAdminInheritsUser(t *testing.T) {
	e := testEnforcer(t)

	for _, p := range defaultPolicies {
		if p[0] != "user" {
			continue
		}
		obj, act := p[1], p[2]
		allowed, err := e.Enforce("admin", obj, act)
		if err != nil {
			t.Fatalf("enforce(admin, %q, %q): %v", obj, act, err)
		}
		if !allowed {
			t.Errorf("admin should inherit user permission on %q/%q but was denied", obj, act)
		}
	}
}

// TestSeedIsIdempotent verifies seeding twice does not duplicate policy lines,
// matching the "seed only when empty" guard used on every boot.
func TestSeedIsIdempotent(t *testing.T) {
	e := testEnforcer(t) // already seeded once

	before, err := e.GetPolicy()
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if err := seed(e); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	after, err := e.GetPolicy()
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("policy count changed after re-seed: before %d, after %d", len(before), len(after))
	}
}
