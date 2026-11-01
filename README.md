# NaviServer QA Smoke Plugin

A harmless, test-only external plugin for verifying GitHub Release installation and the NaviServer plugin-host handshake in an isolated QA instance. It registers one inert game type, declares no UI or operations, and requests no game capabilities. It does not open files, access the network, or launch processes. Like every NaviServer plugin, it runs with the daemon account's permissions.

This repository is intentionally separate from NaviServer and is intended only for a disposable local QA database. The release currently targets Linux/amd64.

## Build

- `go test ./...`
- `./scripts/package-linux-amd64.sh`

The NaviServer Go SDK source is vendored in `third_party/plugin-sdk` from the corresponding SDK module in the NaviServer repository; its license is included there.

## Release assets

The installer expects exactly `qa-smoke_1.0.0_linux_amd64.zip` and its `.sha256` companion. The ZIP root contains `manifest.json` and the `plugin` executable. The checksum detects corruption; it is not author authentication.
