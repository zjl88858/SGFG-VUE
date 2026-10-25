package main

import (
	"encoding/xml"
	"os"
	"testing"
)

func TestGUIAndHelperHaveSeparateElevationManifests(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"build/windows/wails.exe.manifest", "asInvoker"},
		{"build/windows/helper.exe.manifest", "requireAdministrator"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Trust struct {
					Security struct {
						Privileges struct {
							Execution struct {
								Level string `xml:"level,attr"`
							} `xml:"requestedExecutionLevel"`
						} `xml:"requestedPrivileges"`
					} `xml:"security"`
				} `xml:"trustInfo"`
			}
			if err := xml.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Trust.Security.Privileges.Execution.Level != tc.want {
				t.Fatalf("wrong privilege boundary: expected %s", tc.want)
			}
		})
	}
}
