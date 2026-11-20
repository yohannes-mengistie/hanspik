// Package bootstrap creates the first admin account. It runs at startup so a
// fresh deployment is usable without direct database access: the admin-only
// API cannot be called until an admin already exists.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/config"
	"github.com/yohannes/chines-backend/internal/user"
)

// Admin creates or repairs the admin account from BOOTSTRAP_ADMIN_* settings.
// It is a no-op unless both the email and password are set. For an existing
// account it also clears "disabled" and revokes old sessions, so the supplied
// credentials always work after a redeploy.
func Admin(ctx context.Context, cfg config.Config, users *user.Repository, tokens *auth.Repository) error {
	email := strings.ToLower(strings.TrimSpace(cfg.BootstrapAdminEmail))
	password := cfg.BootstrapAdminPassword
	if email == "" && password == "" {
		log.Println("bootstrap admin: not configured, skipping")
		return nil
	}
	if email == "" || password == "" {
		return errors.New("set both BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD")
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL is not a valid email address")
	}
	if len(password) < 8 || len(password) > 72 {
		return errors.New("BOOTSTRAP_ADMIN_PASSWORD must be 8 to 72 characters")
	}

	name := strings.TrimSpace(cfg.BootstrapAdminName)
	if name == "" {
		name = "Admin"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	u, err := users.GetByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		if _, err := users.Create(ctx, name, email, string(hash), user.RoleAdmin); err != nil {
			return fmt.Errorf("create bootstrap admin: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("look up bootstrap admin: %w", err)
	} else {
		if err := users.SetRole(ctx, u.ID, user.RoleAdmin); err != nil {
			return fmt.Errorf("promote bootstrap admin: %w", err)
		}
		if err := users.SetPassword(ctx, u.ID, string(hash)); err != nil {
			return fmt.Errorf("set bootstrap admin password: %w", err)
		}
		if err := users.SetDisabled(ctx, u.ID, false); err != nil {
			return fmt.Errorf("enable bootstrap admin: %w", err)
		}
		if err := tokens.RevokeAllForUser(ctx, u.ID); err != nil {
			return fmt.Errorf("revoke old admin sessions: %w", err)
		}
	}
	log.Printf("bootstrap admin ready: %s", email)
	return nil
}
