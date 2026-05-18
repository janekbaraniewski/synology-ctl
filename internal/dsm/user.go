package dsm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// User is a DSM local user account.
type User struct {
	Name                string   `json:"name"`
	UID                 int      `json:"uid"`
	Description         string   `json:"description"`
	Email               string   `json:"email"`
	Expired             string   `json:"expired"` // "now", "normal", or a date
	Groups              []string `json:"groups,omitempty"`
	PasswordNeverExpire bool     `json:"password_never_expire,omitempty"`
}

// Users returns all local users. additional may include "email", "description",
// "expired", "cannot_chg_passwd", "password_last_change", "user_2step_status".
func (c *Client) Users(ctx context.Context) ([]User, error) {
	params := url.Values{}
	params.Set("type", "local")
	params.Set("offset", "0")
	params.Set("limit", "-1")
	params.Set("additional", `["email","description","expired","cannot_chg_passwd","password_last_change","user_2step_status"]`)
	var resp struct {
		Users  []User `json:"users"`
		Offset int    `json:"offset"`
		Total  int    `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.User", 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Users, nil
}

// Group is a DSM local group.
type Group struct {
	Name        string `json:"name"`
	GID         int    `json:"gid"`
	Description string `json:"description"`
}

// Groups returns all local groups. DSM 7 exposes this under
// SYNO.Core.User.Group (not SYNO.Core.Group, which doesn't exist).
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	params := url.Values{}
	params.Set("type", "local")
	params.Set("offset", "0")
	params.Set("limit", "-1")
	params.Set("additional", `["description"]`)
	var resp struct {
		Groups []Group `json:"groups"`
		Offset int     `json:"offset"`
		Total  int     `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.User.Group", 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Groups, nil
}

// NewUser bundles the fields needed to create a local DSM account.
// Description and Email are optional; Expired defaults to "normal"
// when empty (DSM's "account is active" sentinel). Password is
// required — DSM rejects creation without one.
type NewUser struct {
	Name                string
	Password            string
	Description         string
	Email               string
	Expired             string // "normal", "now", or a YYYY-MM-DD date — "" → "normal"
	PasswordNeverExpire bool
}

// UserPatch is a sparse update for an existing user. Only the pointers
// that are non-nil get sent in the `set` call — DSM's set endpoint
// otherwise wants every field round-tripped, which is brittle when
// firmware adds new fields. The pointer-per-field shape lets the
// caller express "leave this alone" cleanly.
//
// Password is a special case: it goes through SetUserPassword which
// uses the same `set` endpoint but is exposed as its own method so
// the TUI can route it through a dedicated "change password" flow
// without having to construct a UserPatch.
type UserPatch struct {
	Description         *string
	Email               *string
	Expired             *string
	PasswordNeverExpire *bool
}

// CreateUser provisions a local user. The DSM endpoint is
// SYNO.Core.User v1 `create`. The account lands disabled if the
// caller doesn't supply a password — we therefore validate that
// up front rather than letting DSM produce a generic 400.
//
// Group membership cannot be set in this call: DSM exposes it
// through SYNO.Core.Group.Member separately, and adding it here
// would force every caller to either supply or omit a list. The
// TUI's user form covers create-without-groups; "add user to
// groups" stays a follow-up action.
func (c *Client) CreateUser(ctx context.Context, u NewUser) error {
	if u.Name == "" {
		return fmt.Errorf("dsm: user name is required")
	}
	if u.Password == "" {
		return fmt.Errorf("dsm: password is required to create a user")
	}
	expired := u.Expired
	if expired == "" {
		expired = "normal"
	}
	params := url.Values{}
	params.Set("name", u.Name)
	params.Set("password", u.Password)
	params.Set("description", u.Description)
	params.Set("email", u.Email)
	params.Set("expired", expired)
	params.Set("password_never_expire", strconv.FormatBool(u.PasswordNeverExpire))
	return c.Call(ctx, "SYNO.Core.User", 1, "create", params, nil)
}

// UpdateUser applies a sparse patch to an existing user via the
// SYNO.Core.User v1 `set` endpoint. Only the fields whose pointer
// is non-nil are sent; this avoids the "round-trip every field"
// trap noted in the README.
func (c *Client) UpdateUser(ctx context.Context, name string, patch UserPatch) error {
	if name == "" {
		return fmt.Errorf("dsm: user name is required")
	}
	params := url.Values{}
	params.Set("name", name)
	if patch.Description != nil {
		params.Set("description", *patch.Description)
	}
	if patch.Email != nil {
		params.Set("email", *patch.Email)
	}
	if patch.Expired != nil {
		params.Set("expired", *patch.Expired)
	}
	if patch.PasswordNeverExpire != nil {
		params.Set("password_never_expire", strconv.FormatBool(*patch.PasswordNeverExpire))
	}
	return c.Call(ctx, "SYNO.Core.User", 1, "set", params, nil)
}

// SetUserPassword resets a user's password. DSM piggy-backs this on
// the same `set` endpoint but expects only the name+password pair;
// sending stray fields can flip other attributes by accident.
func (c *Client) SetUserPassword(ctx context.Context, name, password string) error {
	if name == "" {
		return fmt.Errorf("dsm: user name is required")
	}
	if password == "" {
		return fmt.Errorf("dsm: password is required")
	}
	params := url.Values{}
	params.Set("name", name)
	params.Set("password", password)
	return c.Call(ctx, "SYNO.Core.User", 1, "set", params, nil)
}

// DeleteUser removes a local user via SYNO.Core.User v1 `delete`.
// The DSM endpoint expects a JSON-array-encoded name list even for
// a single account; we wrap the lone name here rather than push
// that quoting onto callers.
func (c *Client) DeleteUser(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("dsm: user name is required")
	}
	params := url.Values{}
	params.Set("name", `["`+name+`"]`)
	return c.Call(ctx, "SYNO.Core.User", 1, "delete", params, nil)
}
