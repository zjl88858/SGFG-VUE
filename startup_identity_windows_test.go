//go:build windows

package main

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestGUIStartupIdentityPolicy(t *testing.T) {
	const (
		user     = "S-1-5-21-1-2-3-1001"
		admin    = "S-1-5-21-1-2-3-500"
		isolated = "S-1-5-21-1-2-3-1002"
	)
	unavailable := errors.New("desktop unavailable")
	for _, tc := range []struct {
		name             string
		current, desktop startupIdentity
		desktopErr, want error
		queryDesktop     bool
	}{
		{"ordinary user without shell", startupIdentity{user, 1, false}, startupIdentity{}, unavailable, nil, false},
		{"Administrator direct logon", startupIdentity{admin, 1, true}, startupIdentity{admin, 1, true}, nil, nil, true},
		{"renamed administrator or UAC disabled", startupIdentity{user, 1, true}, startupIdentity{user, 1, true}, nil, nil, true},
		{"administrator desktop over RDP", startupIdentity{admin, 7, true}, startupIdentity{admin, 7, true}, nil, nil, true},
		{"Administrator Protection isolated account", startupIdentity{isolated, 1, true}, startupIdentity{user, 1, false}, nil, errStartupAccountMismatch, true},
		{"run as different administrator", startupIdentity{admin, 1, true}, startupIdentity{user, 1, true}, nil, errStartupAccountMismatch, true},
		{"manual elevation with same SID", startupIdentity{user, 1, true}, startupIdentity{user, 1, false}, nil, errStartupElevationMismatch, true},
		{"same SID from another session", startupIdentity{admin, 7, true}, startupIdentity{admin, 1, true}, nil, errStartupAccountMismatch, true},
		{"service session", startupIdentity{admin, 0, true}, startupIdentity{admin, 0, true}, nil, errStartupAccountMismatch, true},
		{"missing SID", startupIdentity{"", 1, true}, startupIdentity{"", 1, true}, nil, errStartupAccountMismatch, true},
		{"elevated desktop query failure", startupIdentity{admin, 1, true}, startupIdentity{}, unavailable, unavailable, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queried := false
			err := checkGUIStartupIdentity(tc.current, func() (startupIdentity, error) {
				queried = true
				return tc.desktop, tc.desktopErr
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if queried != tc.queryDesktop {
				t.Fatalf("desktop queried = %v, want %v", queried, tc.queryDesktop)
			}
		})
	}
}

// Read-only Win32 integration: no UI, UAC, registry writes or driver access.
func TestStartupIdentityTokenQueries(t *testing.T) {
	current, err := readStartupIdentity(windows.GetCurrentProcessToken())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := windows.StringToSid(current.userSID); err != nil {
		t.Fatal("invalid token user SID")
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil {
		t.Fatal(err)
	}
	if current.sessionID != session || current.elevated != windows.GetCurrentProcessToken().IsElevated() {
		t.Fatal("token identity does not match current process")
	}
	if _, err := readStartupIdentity(windows.Token(0)); err == nil {
		t.Fatal("invalid token was treated as an ordinary user")
	}
}

func TestStartupIdentityDesktopQuery(t *testing.T) {
	if windows.GetShellWindow() == 0 {
		t.Skip("no interactive desktop shell")
	}
	desktop, err := desktopStartupIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := windows.StringToSid(desktop.userSID); err != nil || desktop.sessionID == 0 {
		t.Fatal("invalid desktop identity")
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil {
		t.Fatal(err)
	}
	if desktop.sessionID != session {
		t.Fatal("queried a shell from another Windows session")
	}
}
