package kroger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// token is an OAuth access token with its refresh token and expiry.
type token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}

// valid reports whether the token can be used at now, leaving a minute of
// slack so a token doesn't expire mid-request.
func (t *token) valid(now time.Time) bool {
	return t != nil && t.AccessToken != "" && now.Add(time.Minute).Before(t.Expiry)
}

// state is everything that must survive a restart.
type state struct {
	UserToken *token `json:"user_token,omitempty"`
	// ProfileID is the linked shopper's Kroger profile, recorded after each
	// link so a later link can tell whether it replaced a different account.
	ProfileID           string `json:"profile_id,omitempty"`
	PreferredLocationID string `json:"preferred_location_id,omitempty"`
}

// stateFile persists state as JSON with owner-only permissions. It holds the
// shopper's refresh token, so it is written 0600 and replaced atomically.
type stateFile struct {
	path string

	mu sync.Mutex
	s  state
}

func loadState(path string) (*stateFile, error) {
	f := &stateFile{path: path}
	b, err := os.ReadFile(path) //nolint:gosec // G304: path is the operator-configured data dir, not user input.
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(b, &f.s); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", path, err)
	}
	return f, nil
}

func (f *stateFile) get() state {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.s
}

// update applies fn to a copy of the state and persists it. The in-memory
// state only changes if the write succeeds.
func (f *stateFile) update(fn func(*state)) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	next := f.s
	fn(&next)

	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".state-*.json")
	if err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	f.s = next
	return nil
}
