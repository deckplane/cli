package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"

	"github.com/deckplane/cli/internal/docker"
	"github.com/deckplane/cli/internal/ui"
)

// Core's API is only reachable from inside its container in domain mode, so
// requests are made there with the Bun runtime the image already ships.
// The request travels on stdin to keep passwords out of process lists.
const requestScript = `const r=JSON.parse(await Bun.stdin.text());` +
	`const res=await fetch("http://localhost:3000"+r.path,{method:r.method,headers:{"content-type":"application/json"},body:r.body?JSON.stringify(r.body):undefined});` +
	`console.log(JSON.stringify({status:res.status,body:await res.text()}))`

func coreRequest(p docker.Project, method, path string, body any) (int, []byte, error) {
	req, _ := json.Marshal(map[string]any{"method": method, "path": path, "body": body})
	out, err := p.Exec(serviceName, bytes.NewReader(req), "bun", "-e", requestScript)
	if err != nil {
		return 0, nil, err
	}
	var res struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &res); err != nil {
		return 0, nil, fmt.Errorf("unexpected response from Core: %s", out)
	}
	return res.Status, []byte(res.Body), nil
}

func setupRequired(p docker.Project) (bool, error) {
	status, body, err := coreRequest(p, "GET", "/api/v1/setup/status", nil)
	if err != nil {
		return false, err
	}
	if status != 200 {
		return false, fmt.Errorf("setup status returned HTTP %d", status)
	}
	var res struct {
		SetupRequired bool `json:"setupRequired"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return false, err
	}
	return res.SetupRequired, nil
}

// Admin is the first administrator account.
type Admin struct {
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Password string `json:"password"`
}

func createAdmin(p docker.Project, a Admin) error {
	status, body, err := coreRequest(p, "POST", "/api/v1/setup/initialize", a)
	if err != nil {
		return err
	}
	if status >= 300 {
		var res struct {
			Message string `json:"message"`
			Error   any    `json:"error"`
		}
		_ = json.Unmarshal(body, &res)
		if res.Message != "" {
			return errors.New(res.Message)
		}
		return fmt.Errorf("Core returned HTTP %d", status)
	}
	return nil
}

// offerAdminSetup creates the first admin at the end of install, from flags
// (non-interactive) or by asking. It reports whether an admin now exists.
func offerAdminSetup(p docker.Project, o InstallOptions, interactive bool) (bool, error) {
	required, err := setupRequired(p)
	if err != nil || !required {
		return !required, err
	}
	if o.AdminPassword == "" {
		o.AdminPassword = os.Getenv("DECKPLANE_ADMIN_PASSWORD")
	}
	if o.AdminEmail != "" && o.AdminPassword != "" {
		a := Admin{Email: o.AdminEmail, FullName: o.AdminName, Password: o.AdminPassword}
		if a.FullName == "" {
			a.FullName = "Administrator"
		}
		if err := validateAdmin(a); err != nil {
			return false, err
		}
		if err := createAdmin(p, a); err != nil {
			return false, err
		}
		ui.Success("Admin account %s created", a.Email)
		return true, nil
	}
	if !interactive {
		return false, nil
	}
	ui.Println("")
	ok, err := ui.Confirm("Create your admin account now?", true)
	if err != nil || !ok {
		return false, err
	}
	return true, SetupAdmin(p)
}

// SetupAdmin asks for the first admin's details and creates the account.
func SetupAdmin(p docker.Project) error {
	a := Admin{}
	var err error
	if a.Email, err = ui.AskRequired("Email:", "", func(v string) error {
		_, err := mail.ParseAddress(v)
		if err != nil {
			return errors.New("enter a valid email address")
		}
		return nil
	}); err != nil {
		return err
	}
	if a.FullName, err = ui.AskRequired("Full name:", "", minLen("name", 2)); err != nil {
		return err
	}
	if a.Password, err = askPassword("Password"); err != nil {
		return err
	}
	if err := createAdmin(p, a); err != nil {
		return err
	}
	ui.Success("Admin account %s created", a.Email)
	return nil
}

func validateAdmin(a Admin) error {
	if _, err := mail.ParseAddress(a.Email); err != nil {
		return fmt.Errorf("invalid admin email %q", a.Email)
	}
	if err := minLen("password", 8)(a.Password); err != nil {
		return err
	}
	return minLen("name", 2)(a.FullName)
}

func minLen(what string, n int) func(string) error {
	return func(v string) error {
		if len([]rune(v)) < n {
			return fmt.Errorf("the %s must be at least %d characters", what, n)
		}
		return nil
	}
}
