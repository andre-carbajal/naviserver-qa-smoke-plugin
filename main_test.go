package main

import (
	"testing"

	sdk "github.com/andre-carbajal/NaviServer/plugin-sdk"
)

func TestSmokeManifestIsValidForSupportedHost(t *testing.T) {
	m := manifest()
	if err := sdk.ValidateManifest(m); err != nil {
		t.Fatalf("invalid package manifest: %v", err)
	}
	if !sdk.SupportsPlatform(m, "linux", "amd64") {
		t.Fatal("QA release must target the local Linux/amd64 host")
	}
	c := m.Types[0].Capabilities
	if len(m.Operations) != 0 || len(m.UI) != 0 || c.Console || c.Files || c.Backups || c.Players || c.Addons || len(c.Metrics) != 0 {
		t.Fatal("smoke plugin must not expose operations, UI or game capabilities")
	}
}
