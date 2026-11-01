package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	sdk "github.com/andre-carbajal/NaviServer/plugin-sdk"
)

func manifest() sdk.Manifest {
	return sdk.Manifest{
		SchemaVersion:   sdk.SchemaVersion,
		ID:              "qa-smoke",
		Name:            "NaviServer QA Smoke Plugin",
		Version:         "1.0.0",
		NaviServerRange: "*",
		ProtocolMajor:   sdk.ProtocolMajor,
		Platforms:       []sdk.Platform{{OS: "linux", Arch: "amd64"}},
		EntryPoint:      "plugin",
		Types: []sdk.ServerType{{
			ID: "qa-smoke.game", Name: "QA Smoke Type",
			Capabilities: sdk.Capabilities{},
			Configuration: sdk.Schema{Type: "object"},
		}},
	}
}

func main() {
	m := manifest()
	if len(os.Args) == 3 && os.Args[1] == "--write-manifest" {
		data, err := json.MarshalIndent(m, "", "  ")
		if err == nil {
			err = os.WriteFile(os.Args[2], append(data, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := sdk.ValidateManifest(m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err := sdk.Serve(context.Background(), os.Stdin, os.Stdout, func(_ context.Context, method string, _ json.RawMessage) (any, error) {
		switch method {
		case "handshake":
			return m, nil
		case "reconcile":
			return map[string]bool{"ack": true}, nil
		default:
			return nil, fmt.Errorf("unsupported operation %s", method)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
