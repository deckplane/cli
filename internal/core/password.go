package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deckplane/cli/internal/docker"
	"github.com/deckplane/cli/internal/ui"
)

// Core has no API to reset a forgotten password, so the users table is read
// and updated from inside the Core container, with its DATABASE_URL and the
// Bun runtime. The hash is argon2id with Core's own cost settings
// (src/lib/password.ts), so Core verifies it like one it made itself.
// The request travels on stdin to keep passwords out of process lists.
const usersScript = `const r=JSON.parse(await Bun.stdin.text());` +
	`const sql=new Bun.SQL(process.env.DATABASE_URL);` +
	`let rows;` +
	`if(r.action==="list"){rows=await sql` + "`" + `select email, full_name as "fullName", auth_provider as "authProvider", is_active as "isActive" from users order by created_at` + "`" + `}` +
	`else{const hash=await Bun.password.hash(r.password,{algorithm:"argon2id",memoryCost:65536,timeCost:3});` +
	`rows=await sql` + "`" + `update users set password_hash=${hash}, updated_at=now() where email=${r.email} and auth_provider='local' returning email` + "`" + `}` +
	`await sql.close();` +
	`console.log(JSON.stringify([...rows]))`

type coreUser struct {
	Email        string `json:"email"`
	FullName     string `json:"fullName"`
	AuthProvider string `json:"authProvider"`
	IsActive     bool   `json:"isActive"`
}

func usersRequest(p docker.Project, req map[string]string, out any) error {
	body, _ := json.Marshal(req)
	res, err := p.Exec(serviceName, bytes.NewReader(body), "bun", "-e", usersScript)
	if err != nil {
		return ui.NewError("Could not reach Deckplane Core", "Is it running? Check with `deckplane core status`.", err)
	}
	if err := json.Unmarshal(bytes.TrimSpace(res), out); err != nil {
		return fmt.Errorf("unexpected response from Core: %s", res)
	}
	return nil
}

// ResetPassword sets a new password for a local account. Without email it
// asks which account, or picks the only one.
func (in *Installation) ResetPassword(email string) error {
	var users []coreUser
	if err := usersRequest(in.Project, map[string]string{"action": "list"}, &users); err != nil {
		return err
	}
	var local []coreUser
	for _, u := range users {
		if u.AuthProvider == "local" {
			local = append(local, u)
		}
	}
	if len(local) == 0 {
		return ui.NewError("There is no account with a password",
			"Create the first admin with:  deckplane core setup\nAccounts that sign in with SSO change their password there.", nil)
	}

	user, err := pickUser(local, email)
	if err != nil {
		return err
	}
	password, err := newPassword()
	if err != nil {
		return err
	}

	var updated []coreUser
	if err := usersRequest(in.Project, map[string]string{"action": "reset", "email": user.Email, "password": password}, &updated); err != nil {
		return err
	}
	if len(updated) == 0 {
		return fmt.Errorf("the account %s was not updated", user.Email)
	}
	ui.Success("Password changed for %s — sign in at %s", user.Email, in.Settings().URL())
	if !user.IsActive {
		ui.Warn("This account is deactivated. Another admin has to activate it before it can sign in.")
	}
	return nil
}

func pickUser(local []coreUser, email string) (coreUser, error) {
	if email != "" {
		for _, u := range local {
			if strings.EqualFold(u.Email, email) {
				return u, nil
			}
		}
		emails := make([]string, len(local))
		for i, u := range local {
			emails[i] = u.Email
		}
		return coreUser{}, ui.NewError(fmt.Sprintf("No account with a password uses %s", email),
			"Accounts:  "+strings.Join(emails, ", "), nil)
	}
	if len(local) == 1 {
		ui.Info("Account: %s (%s)", local[0].Email, local[0].FullName)
		return local[0], nil
	}
	if !ui.Interactive() {
		return coreUser{}, ui.NewError("More than one account exists", "Pass the email:  deckplane core reset-password EMAIL", nil)
	}
	options := make([]string, len(local))
	for i, u := range local {
		options[i] = fmt.Sprintf("%s (%s)", u.Email, u.FullName)
	}
	i, err := ui.Choose("Which account?", options, 0)
	if err != nil {
		return coreUser{}, err
	}
	return local[i], nil
}

// newPassword asks for the password twice, or reads DECKPLANE_ADMIN_PASSWORD
// when there is no terminal.
func newPassword() (string, error) {
	if !ui.Interactive() {
		password := os.Getenv("DECKPLANE_ADMIN_PASSWORD")
		if password == "" {
			return "", ui.NewError("No terminal to ask for the new password", "Set DECKPLANE_ADMIN_PASSWORD and run the command again.", nil)
		}
		return password, minLen("password", 8)(password)
	}
	return askPassword("New password")
}

// askPassword asks for a password twice until both match.
func askPassword(label string) (string, error) {
	for {
		password, err := ui.AskSecret(label+" (at least 8 characters, hidden):", minLen("password", 8))
		if err != nil {
			return "", err
		}
		again, err := ui.AskSecret("Repeat the password:", nil)
		if err != nil {
			return "", err
		}
		if again == password {
			return password, nil
		}
		ui.Warn("The passwords do not match, try again.")
	}
}
