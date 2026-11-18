package account_types

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

// fakeAPI is an in-memory Sigma: account types, their permissions and members.
type fakeAPI struct {
	types      map[string]*accountType // id -> type
	perms      map[string][]string
	members    map[string]string // memberId -> memberType (type name)
	nextID     int
	failMoveAt int // fail the Nth setMemberType call (1-based); 0 = never
	failDelete bool
	moves      int
	log        []string
	afterList  func() // runs once, right after the first membersOfType returns (simulates a concurrent add)
}

func newFake() *fakeAPI {
	return &fakeAPI{types: map[string]*accountType{}, perms: map[string][]string{}, members: map[string]string{}}
}

func (f *fakeAPI) addType(name string, perms ...string) string {
	f.nextID++
	id := "id-" + name
	f.types[id] = &accountType{AccountTypeID: id, AccountTypeName: name, IsCustom: true}
	f.perms[id] = perms
	return id
}

func (f *fakeAPI) listTypes(context.Context) ([]accountType, error) {
	var out []accountType
	for _, t := range f.types {
		out = append(out, *t)
	}
	return out, nil
}
func (f *fakeAPI) createType(_ context.Context, b createAccountTypeReq) (accountType, error) {
	f.log = append(f.log, "create "+b.Name)
	id := f.addType(b.Name, b.Permissions...)
	f.types[id].Description = b.Description
	return *f.types[id], nil
}
func (f *fakeAPI) typePermissions(_ context.Context, id string) ([]string, error) {
	return append([]string(nil), f.perms[id]...), nil
}
func (f *fakeAPI) patchPermissions(_ context.Context, id string, add, remove []string) error {
	f.log = append(f.log, "patch-perms "+id)
	set := map[string]bool{}
	for _, p := range f.perms[id] {
		set[p] = true
	}
	for _, p := range add {
		set[p] = true
	}
	for _, p := range remove {
		delete(set, p)
	}
	f.perms[id] = nil
	for p := range set {
		f.perms[id] = append(f.perms[id], p)
	}
	return nil
}
func (f *fakeAPI) membersOfType(_ context.Context, name string) ([]memberRef, error) {
	var out []memberRef
	for id, t := range f.members {
		if t == name {
			out = append(out, memberRef{MemberID: id, MemberType: t, Email: id + "@example.invalid"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MemberID < out[j].MemberID })
	if f.afterList != nil {
		fn := f.afterList
		f.afterList = nil
		fn()
	}
	return out, nil
}
func (f *fakeAPI) setMemberType(_ context.Context, id, name string) error {
	f.moves++
	if f.failMoveAt > 0 && f.moves == f.failMoveAt {
		return errors.New("boom")
	}
	f.members[id] = name
	return nil
}
func (f *fakeAPI) deleteType(_ context.Context, id, reassignTo string) error {
	if f.failDelete {
		return errors.New("delete boom")
	}
	f.log = append(f.log, "delete "+id)
	oldName := f.types[id].AccountTypeName
	for m, t := range f.members { // Sigma moves stragglers to the fallback
		if t == oldName {
			f.members[m] = "view"
		}
	}
	delete(f.types, id)
	return nil
}

func in(old, newName string) renameInput {
	return renameInput{OldID: "id-" + old, OldName: old, NewName: newName, NewDescription: "d", Permissions: []string{"a", "b"}, ReassignTo: "view-id"}
}

func TestMigrate_movesMembersThenDeletesOld(t *testing.T) {
	f := newFake()
	f.addType("old", "a", "b")
	f.members = map[string]string{"m1": "old", "m2": "old", "m3": "other"}
	got, err := migrateAccountType(context.Background(), f, in("old", "new"))
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountTypeName != "new" || got.AccountTypeID == "id-old" {
		t.Fatalf("unexpected result %+v", got)
	}
	if f.members["m1"] != "new" || f.members["m2"] != "new" || f.members["m3"] != "other" {
		t.Fatalf("members not migrated correctly: %v", f.members)
	}
	if _, ok := f.types["id-old"]; ok {
		t.Fatal("old type should be deleted")
	}
	for _, m := range f.members {
		if m == "view" {
			t.Fatal("nobody should fall to the delete fallback")
		}
	}
}

func TestMigrate_failedMoveKeepsOldTypeAndResumes(t *testing.T) {
	f := newFake()
	f.addType("old", "a", "b")
	f.members = map[string]string{"m1": "old", "m2": "old", "m3": "old"}
	f.failMoveAt = 2
	_, err := migrateAccountType(context.Background(), f, in("old", "new"))
	if err == nil || !strings.Contains(err.Error(), "re-apply to resume") {
		t.Fatalf("want resumable error, got %v", err)
	}
	if _, ok := f.types["id-old"]; !ok {
		t.Fatal("old type must survive a failed move")
	}
	// Re-apply: the half-built target is reused and the rest are moved.
	f.failMoveAt = 0
	got, err := migrateAccountType(context.Background(), f, in("old", "new"))
	if err != nil {
		t.Fatal(err)
	}
	creates := 0
	for _, l := range f.log {
		if l == "create new" {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("target should be created once, got %d", creates)
	}
	if got.AccountTypeName != "new" {
		t.Fatalf("got %+v", got)
	}
	for m, ty := range f.members {
		if ty != "new" {
			t.Fatalf("%s left on %s", m, ty)
		}
	}
}

func TestMigrate_reusedTargetGetsPermissionsSynced(t *testing.T) {
	f := newFake()
	f.addType("old", "a", "b")
	f.addType("new", "a") // leftover with stale permissions
	if _, err := migrateAccountType(context.Background(), f, in("old", "new")); err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), f.perms["id-new"]...)
	sort.Strings(got)
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("permissions not synced: %v", got)
	}
}

func TestMigrate_memberAddedMidMigrationIsCaughtByNextPass(t *testing.T) {
	f := newFake()
	f.addType("old", "a", "b")
	f.members = map[string]string{"m1": "old"}
	// m2 joins the old type after the first listing, so only a later pass can see it.
	f.afterList = func() { f.members["m2"] = "old" }
	if _, err := migrateAccountType(context.Background(), f, in("old", "new")); err != nil {
		t.Fatal(err)
	}
	if f.members["m1"] != "new" || f.members["m2"] != "new" {
		t.Fatalf("late member not moved: %v", f.members)
	}
}

func TestMigrate_failedDeleteLeavesMembersOnNewAndIsResumable(t *testing.T) {
	f := newFake()
	f.addType("old", "a", "b")
	f.members = map[string]string{"m1": "old"}
	f.failDelete = true
	_, err := migrateAccountType(context.Background(), f, in("old", "new"))
	if err == nil || !strings.Contains(err.Error(), "re-apply to finish") {
		t.Fatalf("want resumable delete error, got %v", err)
	}
	if f.members["m1"] != "new" {
		t.Fatalf("member should already be on the new type: %v", f.members)
	}
	f.failDelete = false
	if _, err := migrateAccountType(context.Background(), f, in("old", "new")); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.types["id-old"]; ok {
		t.Fatal("old type should be gone after re-apply")
	}
}

func TestMigrate_refusesSameNameAndBuiltInTarget(t *testing.T) {
	f := newFake()
	f.addType("old", "a")
	if _, err := migrateAccountType(context.Background(), f, in("old", "old")); err == nil {
		t.Fatal("same name must be rejected")
	}
	f.types["id-builtin"] = &accountType{AccountTypeID: "id-builtin", AccountTypeName: "view", IsCustom: false}
	if _, err := migrateAccountType(context.Background(), f, in("old", "view")); err == nil {
		t.Fatal("built-in target must be rejected")
	}
}
