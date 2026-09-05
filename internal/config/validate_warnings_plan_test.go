package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// basePlanFixture returns a fresh *PlanConfig with every D6-compared field populated,
// so each test can mutate exactly one field off a known-equal baseline. A fresh value
// per call matters: tests must not share a *PlanConfig with another fixture, or a
// mutation in one test would leak into another via the shared pointer.
func basePlanFixture() *PlanConfig {
	return &PlanConfig{
		Environment:  "dev",
		Site:         "site-a",
		Vars:         map[string]string{"a": "1", "b": "2"},
		EndpointTags: []string{"tag1", "tag2"},
		Entries: []PlanEntry{
			{
				Name:      "svc1",
				Runner:    "compose",
				Order:     10,
				DependsOn: []string{"dep1", "dep2"},
				Services:  []string{"web", "db"},
				Vars:      map[string]string{"x": "1", "y": "2"},
			},
		},
	}
}

// TestDuplicatePlanDeclarationsEqualDeclarationsWarn covers the "equal declarations"
// fixture: two distinct *PlanConfig values with identical compared fields must warn
// exactly once, with the pair named in sorted order, and the message must describe
// equal declaration fields without claiming runtime equivalence or recommending a
// canonical name.
func TestDuplicatePlanDeclarationsEqualDeclarationsWarn(t *testing.T) {
	c := &Config{Plans: map[string]*PlanConfig{
		"beta":  basePlanFixture(),
		"alpha": basePlanFixture(),
	}}

	got := c.warnDuplicatePlanDeclarations()
	want := []string{`plans "alpha" and "beta" declare equal environment, site, vars, endpoint_tags, entries, and composes — review whether both are intentional`}
	if !slices.Equal(got, want) {
		t.Fatalf("warnDuplicatePlanDeclarations() = %v\n  want %v", got, want)
	}
}

// TestDuplicatePlanDeclarationsOneFieldDifferenceIsNotDuplicate covers every one-field
// difference the card lists: all four plan fields and all six entry fields. Each case
// must NOT warn — a single differing field is enough to make the declarations distinct.
func TestDuplicatePlanDeclarationsOneFieldDifferenceIsNotDuplicate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PlanConfig)
	}{
		{"plan.environment", func(p *PlanConfig) { p.Environment = "prod" }},
		{"plan.site", func(p *PlanConfig) { p.Site = "site-b" }},
		{"plan.vars", func(p *PlanConfig) { p.Vars["a"] = "different" }},
		{"plan.endpoint_tags", func(p *PlanConfig) { p.EndpointTags = []string{"tag1", "tag3"} }},
		{"entry.name", func(p *PlanConfig) { p.Entries[0].Name = "svc2" }},
		{"entry.runner", func(p *PlanConfig) { p.Entries[0].Runner = "kubectl" }},
		{"entry.order", func(p *PlanConfig) { p.Entries[0].Order = 20 }},
		{"entry.depends_on", func(p *PlanConfig) { p.Entries[0].DependsOn = []string{"dep1", "dep3"} }},
		{"entry.services", func(p *PlanConfig) { p.Entries[0].Services = []string{"web", "cache"} }},
		{"entry.vars", func(p *PlanConfig) { p.Entries[0].Vars["x"] = "different" }},
		{"plan.composes", func(p *PlanConfig) { p.Composes = []CompositionEntry{{Plan: "infra"}} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := basePlanFixture()
			b := basePlanFixture()
			tc.mutate(b)

			if plansHaveEqualDeclaration(a, b) {
				t.Fatalf("plansHaveEqualDeclaration = true after changing %s, want false", tc.name)
			}

			c := &Config{Plans: map[string]*PlanConfig{"a": a, "b": b}}
			if got := c.warnDuplicatePlanDeclarations(); len(got) != 0 {
				t.Fatalf("warnDuplicatePlanDeclarations() = %v, want none (differs by %s)", got, tc.name)
			}
		})
	}
}

// TestDuplicatePlanDeclarationsMapOrderDoesNotMatter covers the "map-order-only"
// fixture: the same key/value pairs built via different insertion order must still be
// a duplicate. Go map iteration order is randomized regardless of insertion order, so
// this pins that the comparison is by content (maps.Equal), not by any incidental
// construction order.
func TestDuplicatePlanDeclarationsMapOrderDoesNotMatter(t *testing.T) {
	a := basePlanFixture()
	a.Vars = map[string]string{"a": "1", "b": "2", "c": "3"}
	a.Entries[0].Vars = map[string]string{"x": "1", "y": "2"}

	b := basePlanFixture()
	b.Vars = map[string]string{}
	for _, kv := range [][2]string{{"c", "3"}, {"a", "1"}, {"b", "2"}} {
		b.Vars[kv[0]] = kv[1]
	}
	b.Entries[0].Vars = map[string]string{}
	for _, kv := range [][2]string{{"y", "2"}, {"x", "1"}} {
		b.Entries[0].Vars[kv[0]] = kv[1]
	}

	if !plansHaveEqualDeclaration(a, b) {
		t.Fatal("plansHaveEqualDeclaration = false for map-order-only difference, want true (duplicate)")
	}
}

// TestDuplicatePlanDeclarationsListOrderDifferenceIsNotDuplicate covers the
// "list-order difference" fixture: the card requires list order to be significant, so
// the same elements in a different order must NOT be a duplicate — for both a
// plan-level slice (EndpointTags) and an entry-level slice (DependsOn).
func TestDuplicatePlanDeclarationsListOrderDifferenceIsNotDuplicate(t *testing.T) {
	a := basePlanFixture()
	b := basePlanFixture()
	b.EndpointTags = []string{"tag2", "tag1"}
	if plansHaveEqualDeclaration(a, b) {
		t.Error("endpoint_tags reordering treated as duplicate, want distinct")
	}

	a2 := basePlanFixture()
	b2 := basePlanFixture()
	b2.Entries[0].DependsOn = []string{"dep2", "dep1"}
	if plansHaveEqualDeclaration(a2, b2) {
		t.Error("entry depends_on reordering treated as duplicate, want distinct")
	}
}

// TestDuplicatePlanDeclarationsNilAndEmptyCollectionsAreEqual covers the
// "nil/empty equality" fixture: a plan/entry that omits a map or slice entirely must
// compare equal to one that declares it as an explicit empty collection.
func TestDuplicatePlanDeclarationsNilAndEmptyCollectionsAreEqual(t *testing.T) {
	nilForm := &PlanConfig{
		Environment: "dev",
		Entries: []PlanEntry{
			{Name: "svc1", Runner: "compose", Order: 10},
		},
	}
	emptyForm := &PlanConfig{
		Environment:  "dev",
		Vars:         map[string]string{},
		EndpointTags: []string{},
		Entries: []PlanEntry{
			{
				Name: "svc1", Runner: "compose", Order: 10,
				DependsOn: []string{}, Services: []string{}, Vars: map[string]string{},
			},
		},
	}

	if !plansHaveEqualDeclaration(nilForm, emptyForm) {
		t.Fatal("nil and empty collections not treated as equal")
	}
}

// TestDuplicatePlanDeclarationsDoesNotCompareAcrossPartitions covers the "subproject
// namespaces" fixture: root-declared and subproject-imported plans, and two different
// subprojects' plans, must never be compared even when their declarations would
// otherwise be equal — SubprojectPath is the partition key.
func TestDuplicatePlanDeclarationsDoesNotCompareAcrossPartitions(t *testing.T) {
	root := basePlanFixture() // SubprojectPath == "" (root-declared)
	child := basePlanFixture()
	child.SubprojectPath = "/repo/child"

	rootVsChild := &Config{Plans: map[string]*PlanConfig{"root-plan": root, "child/plan": child}}
	if got := rootVsChild.warnDuplicatePlanDeclarations(); len(got) != 0 {
		t.Errorf("root vs child compared across partitions: %v", got)
	}

	childA := basePlanFixture()
	childA.SubprojectPath = "/repo/child-a"
	childB := basePlanFixture()
	childB.SubprojectPath = "/repo/child-b"

	childAVsChildB := &Config{Plans: map[string]*PlanConfig{"child-a/plan": childA, "child-b/plan": childB}}
	if got := childAVsChildB.warnDuplicatePlanDeclarations(); len(got) != 0 {
		t.Errorf("child-a vs child-b compared across partitions: %v", got)
	}
}

// TestDuplicatePlanDeclarationsWarnsWithinSameSubprojectPartition proves the
// partition check is not simply "never warn on imported plans" — two equal
// declarations imported from the SAME subproject (equal SubprojectPath) must still
// warn.
func TestDuplicatePlanDeclarationsWarnsWithinSameSubprojectPartition(t *testing.T) {
	a := basePlanFixture()
	a.SubprojectPath = "/repo/child"
	b := basePlanFixture()
	b.SubprojectPath = "/repo/child"

	c := &Config{Plans: map[string]*PlanConfig{"child/one": a, "child/two": b}}
	if got := c.warnDuplicatePlanDeclarations(); len(got) != 1 {
		t.Fatalf("warnDuplicatePlanDeclarations() = %v, want exactly 1 warning", got)
	}
}

// TestDuplicatePlanDeclarationsExcludesRealAliasImport covers the pointer-identity
// exclusion through the real import path (not a hand-built pointer alias): a
// subproject plan imported with `as:` produces two map keys — the canonical
// `child/dev` and the alias `dev-alias` — that hold the SAME *PlanConfig. That pair
// must never be reported as a duplicate.
func TestDuplicatePlanDeclarationsExcludesRealAliasImport(t *testing.T) {
	workspace := t.TempDir()
	parentDir := filepath.Join(workspace, "parent")
	childDir := filepath.Join(workspace, "child")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, FileName), []byte(`
version: "0.1.0"
plans:
  dev: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  child:
    path: ../child
    import:
      plans:
        - name: dev
          as: dev-alias
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(parentDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	canonical := cfg.Plans["child/dev"]
	alias := cfg.Plans["dev-alias"]
	if canonical == nil || alias == nil {
		t.Fatalf("expected both canonical and alias plan entries, got canonical=%v alias=%v", canonical, alias)
	}
	if canonical != alias {
		t.Fatal("canonical and alias plans are not the same *PlanConfig pointer; the real import path no longer aliases")
	}

	if got := cfg.warnDuplicatePlanDeclarations(); len(got) != 0 {
		t.Fatalf("warnDuplicatePlanDeclarations() = %v, want none (canonical/alias pointer pair must be excluded)", got)
	}
}

// TestDuplicatePlanDeclarationsAreOrderStable pins deterministic pair ordering: names
// are iterated sorted, so the same three-plan fixture (two of them duplicates) must
// produce the identical warning slice across many runs despite Go's randomized map
// iteration. Mirrors TestInteractionWarningsAreOrderStable /
// TestFlatMapWarningsAreOrderStable.
func TestDuplicatePlanDeclarationsAreOrderStable(t *testing.T) {
	c := &Config{Plans: map[string]*PlanConfig{
		"zeta":  basePlanFixture(),
		"alpha": basePlanFixture(),
		"mid":   {Environment: "prod"}, // distinct, never duplicates the others
	}}

	first := c.warnDuplicatePlanDeclarations()
	if len(first) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(first), first)
	}
	for i := range 50 {
		got := c.warnDuplicatePlanDeclarations()
		if !slices.Equal(got, first) {
			t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
		}
	}
}

// TestMultiplePlansWithoutDefaultWarns covers D7's core case: two or more plans, no
// default_plan, warns and names every plan (sorted).
func TestMultiplePlansWithoutDefaultWarns(t *testing.T) {
	c := &Config{Plans: map[string]*PlanConfig{
		"beta":  {},
		"alpha": {},
	}}

	got := c.warnMultiplePlansWithoutDefault()
	want := []string{"2 plans are defined (alpha, beta) but default_plan is not set — bare lifecycle commands (e.g. 'dva up') require naming a plan explicitly; set default_plan to one of them"}
	if !slices.Equal(got, want) {
		t.Fatalf("warnMultiplePlansWithoutDefault() = %v\n  want %v", got, want)
	}
}

// TestMultiplePlansWithoutDefaultSinglePlanDoesNotWarn covers the excluded
// single-plan implicit-default contract: DefaultPlan() already treats a lone plan as
// the default, so no warning is needed or wanted.
func TestMultiplePlansWithoutDefaultSinglePlanDoesNotWarn(t *testing.T) {
	c := &Config{Plans: map[string]*PlanConfig{"only": {}}}
	if got := c.warnMultiplePlansWithoutDefault(); len(got) != 0 {
		t.Fatalf("warnMultiplePlansWithoutDefault() = %v, want none for a single plan", got)
	}
}

// TestMultiplePlansWithoutDefaultExplicitValidDefaultDoesNotWarn covers an explicit,
// valid default_plan: no ambiguity exists, so no warning.
func TestMultiplePlansWithoutDefaultExplicitValidDefaultDoesNotWarn(t *testing.T) {
	c := &Config{
		Plans:           map[string]*PlanConfig{"alpha": {}, "beta": {}},
		DefaultPlanName: "alpha",
	}
	if got := c.warnMultiplePlansWithoutDefault(); len(got) != 0 {
		t.Fatalf("warnMultiplePlansWithoutDefault() = %v, want none when default_plan is valid", got)
	}
}

// TestMultiplePlansWithoutDefaultInvalidDefaultDoesNotDuplicateHardError documents
// the declared-but-invalid default_plan behavior: DefaultPlanName is non-empty (it
// names a plan that does not exist), so this check must NOT warn — Validate() already
// rejects that case as a hard error, and c.DefaultPlanName == "" (not DefaultPlan()
// or DefaultPlanSource()) is exactly what keeps this check from re-warning on top of
// it.
func TestMultiplePlansWithoutDefaultInvalidDefaultDoesNotDuplicateHardError(t *testing.T) {
	c := &Config{
		Plans:           map[string]*PlanConfig{"alpha": {}, "beta": {}},
		DefaultPlanName: "does-not-exist",
	}
	// Sanity check on the premise: DefaultPlan() resolves to "" here exactly as it
	// would for an absent default_plan, which is why this check cannot use it.
	if got := c.DefaultPlan(); got != "" {
		t.Fatalf("premise broken: DefaultPlan() = %q, want empty for an invalid reference", got)
	}
	if got := c.warnMultiplePlansWithoutDefault(); len(got) != 0 {
		t.Fatalf("warnMultiplePlansWithoutDefault() = %v, want none; Validate() hard-errors this case instead", got)
	}
}

// TestMultiplePlansWithoutDefaultAreOrderStable pins deterministic plan-name
// ordering in the message across many runs of the same map.
func TestMultiplePlansWithoutDefaultAreOrderStable(t *testing.T) {
	c := &Config{Plans: map[string]*PlanConfig{
		"zeta": {}, "alpha": {}, "mid": {}, "beta": {},
	}}

	first := c.warnMultiplePlansWithoutDefault()
	if len(first) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(first), first)
	}
	for i := range 50 {
		got := c.warnMultiplePlansWithoutDefault()
		if !slices.Equal(got, first) {
			t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
		}
	}
}

// TestDuplicatePlanDeclarationsCompositionPlans covers TASK-324: two composition
// plans have no Entries, so before Composes was compared every pair warned. Differing
// Composes (plan, order, depends_on, vars, or list order) must not warn; equal ones must.
func TestDuplicatePlanDeclarationsCompositionPlans(t *testing.T) {
	base := func() *PlanConfig {
		return &PlanConfig{Composes: []CompositionEntry{
			{Plan: "infra", Order: 0},
			{Plan: "backend/dev", Order: 1, DependsOn: []string{"infra"}, Vars: map[string]string{"k": "v"}},
		}}
	}
	cases := []struct {
		name   string
		mutate func(*PlanConfig)
	}{
		{"composes.plan", func(p *PlanConfig) { p.Composes[1].Plan = "frontend/dev" }},
		{"composes.order", func(p *PlanConfig) { p.Composes[1].Order = 2 }},
		{"composes.depends_on", func(p *PlanConfig) { p.Composes[1].DependsOn = nil }},
		{"composes.vars", func(p *PlanConfig) { p.Composes[1].Vars["k"] = "other" }},
		{"composes.length", func(p *PlanConfig) { p.Composes = p.Composes[:1] }},
		{"composes.list-order", func(p *PlanConfig) { p.Composes[0], p.Composes[1] = p.Composes[1], p.Composes[0] }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := base(), base()
			tc.mutate(b)
			if plansHaveEqualDeclaration(a, b) {
				t.Fatalf("plansHaveEqualDeclaration = true after changing %s, want false", tc.name)
			}
			c := &Config{Plans: map[string]*PlanConfig{"a": a, "b": b}}
			if got := c.warnDuplicatePlanDeclarations(); len(got) != 0 {
				t.Fatalf("warnDuplicatePlanDeclarations() = %v, want none", got)
			}
		})
	}

	c := &Config{Plans: map[string]*PlanConfig{"a": base(), "b": base()}}
	if got := c.warnDuplicatePlanDeclarations(); len(got) != 1 {
		t.Fatalf("warnDuplicatePlanDeclarations() = %v, want one warning for equal composition plans", got)
	}
}
