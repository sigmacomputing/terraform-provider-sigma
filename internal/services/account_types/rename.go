package account_types

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

// Sigma has no endpoint to change an account type's name. A rename is emulated
// by creating the new type, moving its members across, then deleting the old
// type, so nobody is ever reassigned to the delete fallback (Viewer).

// memberRef is the slice of a member the migration needs.
type memberRef struct {
	MemberID   string `json:"memberId"`
	MemberType string `json:"memberType"`
	Email      string `json:"email"`
}

// renameAPI is the set of Sigma calls the migration uses; a fake implements it in tests.
type renameAPI interface {
	listTypes(ctx context.Context) ([]accountType, error)
	createType(ctx context.Context, body createAccountTypeReq) (accountType, error)
	typePermissions(ctx context.Context, id string) ([]string, error)
	patchPermissions(ctx context.Context, id string, add, remove []string) error
	membersOfType(ctx context.Context, typeName string) ([]memberRef, error)
	setMemberType(ctx context.Context, memberID, typeName string) error
	deleteType(ctx context.Context, id, reassignTo string) error
}

type renameInput struct {
	OldID, OldName          string
	NewName, NewDescription string
	Permissions             []string
	ReassignTo              string
}

// maxMemberPasses bounds the re-list loop that catches members added to the old type mid-migration.
const maxMemberPasses = 3

// migrateAccountType moves an account type to a new name. Every step is safe to
// re-run, so a failed apply is finished by applying again: an existing type
// with the new name is reused and members already moved are skipped.
func migrateAccountType(ctx context.Context, api renameAPI, in renameInput) (accountType, error) {
	if in.NewName == in.OldName {
		return accountType{}, fmt.Errorf("migrate requires a new name, got %q for both", in.NewName)
	}

	all, err := api.listTypes(ctx)
	if err != nil {
		return accountType{}, fmt.Errorf("list account types: %w", err)
	}
	var target *accountType
	for i := range all {
		if all[i].AccountTypeName == in.NewName {
			target = &all[i]
			break
		}
	}

	if target == nil {
		created, err := api.createType(ctx, createAccountTypeReq{
			Name:        in.NewName,
			Description: in.NewDescription,
			Permissions: in.Permissions,
		})
		if err != nil {
			return accountType{}, fmt.Errorf("create account type %q: %w", in.NewName, err)
		}
		target = &created
	} else {
		if !target.IsCustom {
			return accountType{}, fmt.Errorf("an account type named %q already exists and is built in", in.NewName)
		}
		// Left over from an interrupted earlier attempt: bring its permissions in line.
		have, err := api.typePermissions(ctx, target.AccountTypeID)
		if err != nil {
			return accountType{}, fmt.Errorf("read permissions of %q: %w", in.NewName, err)
		}
		if add, remove := diffPermissions(have, in.Permissions); len(add) > 0 || len(remove) > 0 {
			if err := api.patchPermissions(ctx, target.AccountTypeID, add, remove); err != nil {
				return accountType{}, fmt.Errorf("sync permissions of %q: %w", in.NewName, err)
			}
		}
	}

	for pass := 0; pass < maxMemberPasses; pass++ {
		ms, err := api.membersOfType(ctx, in.OldName)
		if err != nil {
			return accountType{}, fmt.Errorf("list members of %q: %w", in.OldName, err)
		}
		if len(ms) == 0 {
			break
		}
		for _, m := range ms {
			if err := api.setMemberType(ctx, m.MemberID, in.NewName); err != nil {
				return accountType{}, fmt.Errorf("move %s to %q (the old type %q is untouched; re-apply to resume): %w", m.Email, in.NewName, in.OldName, err)
			}
		}
	}

	// Anyone still on the old type (added after the last pass) falls to ReassignTo.
	if err := api.deleteType(ctx, in.OldID, in.ReassignTo); err != nil {
		return accountType{}, fmt.Errorf("delete old account type %q (members are already on %q; re-apply to finish): %w", in.OldName, in.NewName, err)
	}
	return *target, nil
}

// clientRenameAPI implements renameAPI over the Sigma REST client.
type clientRenameAPI struct{ c *client.Client }

func (a clientRenameAPI) listTypes(ctx context.Context) ([]accountType, error) {
	return client.ListAllPageToken[accountType](ctx, a.c, "/v2/accountTypes", client.PageSize(client.MaxPageSize))
}

func (a clientRenameAPI) createType(ctx context.Context, body createAccountTypeReq) (accountType, error) {
	var got accountType
	err := a.c.Do(ctx, "POST", "/v2/accountTypes", body, &got)
	return got, err
}

func (a clientRenameAPI) typePermissions(ctx context.Context, id string) ([]string, error) {
	type permEntry struct {
		Permission string `json:"permission"`
	}
	perms, err := client.ListAllArray[permEntry](ctx, a.c, "/v2/accountTypes/"+client.EscapeID(id)+"/permissions")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(perms))
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if _, dup := seen[p.Permission]; !dup {
			seen[p.Permission] = struct{}{}
			out = append(out, p.Permission)
		}
	}
	return out, nil
}

func (a clientRenameAPI) patchPermissions(ctx context.Context, id string, add, remove []string) error {
	return a.c.Do(ctx, "PATCH", "/v2/accountTypes/"+client.EscapeID(id)+"/permissions", patchPermissionsReq{Add: add, Remove: remove}, nil)
}

func (a clientRenameAPI) membersOfType(ctx context.Context, typeName string) ([]memberRef, error) {
	all, err := client.ListAllPaged[memberRef](ctx, a.c, "/v2.1/members", nil)
	if err != nil {
		return nil, err
	}
	var out []memberRef
	for _, m := range all {
		if m.MemberType == typeName {
			out = append(out, m)
		}
	}
	return out, nil
}

func (a clientRenameAPI) setMemberType(ctx context.Context, memberID, typeName string) error {
	body := struct {
		MemberType string `json:"memberType"`
	}{typeName}
	return a.c.Do(ctx, "PATCH", "/v2/members/"+client.EscapeID(memberID), body, nil)
}

func (a clientRenameAPI) deleteType(ctx context.Context, id, reassignTo string) error {
	err := a.c.Do(ctx, "DELETE", "/v2/accountTypes/"+client.EscapeID(id)+"?reassignToAccountTypeId="+url.QueryEscape(reassignTo), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		return err
	}
	return nil
}
