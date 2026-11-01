# NaviServer plugin SDK (protocol major 1)

This module is the public language-neutral contract. Go is the supported SDK; other languages implement the same wire messages.

## Transport

The daemon starts the package entrypoint as an independent process. Standard input/output are exclusively JSON-RPC 2.0, framed as `Content-Length: <UTF-8 byte count>\r\n\r\n<body>`. Messages are limited to 16 MiB. Send diagnostic output to stderr, never stdout. Requests use positive numeric IDs scoped to the sender; both peers may issue calls simultaneously. `progress` and `cancel` are notifications; requests may complete out of order.

`Serve(ctx, os.Stdin, os.Stdout, handler)` dispatches concurrently. Handlers must synchronize shared resources and obey cancellation. Return the complete manifest for `handshake` and reconcile the supplied server contexts for `reconcile`; reconciliation must not launch or install games. Use `CallCore(ctx, "core.ping", input, &result)` for reverse core calls from a handler. The host exposes only allowlisted services: `core.ping`, plugin-scoped `core.contexts`, or explicit `CoreMethods` callbacks. Unknown services return JSON-RPC method-not-found, and typed `RPCError` codes survive the transport. Use `ReportProgress(ctx, Progress{OperationID: ..., ...})` for correlated progress. Long operations include `operationId` in params; cancellation sends `cancel` with that identity and the current implementation waits up to 30 seconds for the operation response before the host treats plugin-owned data as safe to reconcile. The owner approved this 30-second D-01 acknowledgement timeout on 2026-10-26. A missing acknowledgement returns `ErrCancelUnacknowledged`; the host must not remove or restore files while that provider may still be writing. Reject duplicate active operation IDs.

Host handshake/reconciliation has a 10-second budget. Ordinary calls without an explicit deadline have a 30-second budget. Long-operation callers must supply their own context/deadline. Transport loss interrupts operations and never replays mutations: reconcile effects before explicitly retrying. Plugins must not spawn the indispensable game process: return an execution plan for the core-owned supervisor instead.

## Configuration and compatibility

`ApplyDefaults` copies input rather than mutating it. `ValidateConfiguration` enforces the supported object/array/string/boolean/number/integer subset, required properties, enums and bounds; unknown properties are rejected. `RedactConfiguration` recursively hides schema-declared secrets for diagnostics; storage still contains the original values and plugins remain trusted code.

For example, a non-Java type can declare its own settings instead of inheriting
Java loader/RAM fields:

```json
{
  "type": "object",
  "required": ["worldName", "port"],
  "properties": {
    "worldName": { "type": "string", "minLength": 1, "maxLength": 64 },
    "port": { "type": "integer", "minimum": 1, "maximum": 65535 },
    "accessToken": { "type": "string", "secret": true }
  }
}
```

The daemon applies defaults and validates submitted and returned configuration
against this schema. `secret` marks values for redaction and round-trip
preservation; it does not encrypt storage or sandbox the trusted plugin.

`ValidateManifest` enforces identity, manifest/protocol major, package-relative entrypoint, operation/contribution identity and scope, and explicitly declared non-private UI resources. `SupportsPlatform` checks exact OS/architecture pairs; distribution/minimum OS versions need installer checks. `VersionMatches` accepts exact SemVer, `*`, whitespace-separated comparators and `||` alternatives. Use `>=1.0.0 <2.0.0`, not caret/tilde shorthand.

## Optional recipe provider binding

Recipe schema v1 has no provider binding. A recipe v2 can allow one optional provider with `{ "pluginId": "example-provider", "versionRange": ">=1.0.0 <2.0.0" }`. The plugin manifest stays schema/protocol major 1 and reciprocates with `providerBindings`, each naming a `recipe.<id>` `typeId`, `recipeVersionRange` and offered provider-only capabilities (`players`, `addons`, declared metrics). Offered capabilities must be a subset of the recipe's v2 allowlist. The core exposes their intersection; a missing/incompatible provider contributes no optional UI/capability and cannot disable the recipe.

A bound contribution uses the target recipe `typeId`; `requiresCapabilities` can name `players`, `addons` or `metric:<id>`. Its `operations` must still be declared with the matching scope/permission. Calls include the exact installed `pluginVersion`; recipe-backed calls also carry the immutable recipe reference. The core rechecks binding, recipe pin, contribution, capability and user permission on every request. A provider binding never grants execution-plan ownership or direct process/file control. Backend packages are trusted code, not sandboxed.

A provider-only package may omit `types` when it declares at least one valid `providerBindings` entry. It still requires its platform, entrypoint and any UI/operations it exposes; a manifest with neither owned types nor provider bindings is rejected.

The daemon supervises plugin availability separately from game state, restarts disconnected plugin processes and supplies current contexts. Plugin crashes cannot cancel the core-owned game lifetime. This does not promise daemon-restart survival or a sandbox. The executable has the daemon account permissions.

## Verification

Run `go test -race ./...` from this module. The daemon host tests additionally include a Python wire fixture and independent game process to validate non-Go interoperability and crash/reconnection behavior.

## Packaged global and server screens

Declare packaged HTML in the manifest; use `global` for a plugin-wide page and
`server` plus `typeId` for a game-specific tab. Capability requirements are
checked against the installed type before a tab is offered:

```json
{
  "ui": [
    { "id": "overview", "title": "Overview", "scope": "global", "path": "ui/overview.html" },
    { "id": "players", "title": "Players", "scope": "server", "typeId": "example.game", "path": "ui/players.html", "requiresCapabilities": ["players"] }
  ]
}
```

Bundle every declared path and its relative resources in the release package.
At runtime the authenticated parent fetches only manifest-declared relative
resources and embeds them as data URLs; CSS imports and `url()` references are
resolved through the same allowlist. Remote/absolute references are not loaded.
Each screen is limited to 128 unique resources and 32 MiB total resource bytes;
exceeding either limit fails closed with a contained screen error.
The shell registers the global link at `/extensions/{pluginId}/{contributionId}`
and the server tab on the matching game instance; installed versions and
contribution identity key the asset/bridge lifecycle. Screens have no direct
network or same-origin access and receive only the declared operations through
the parent bridge. See the design's authorization/CSP contract before adding
operations.

## Packaged creation forms (protocol major 1)

A contribution with `scope: "create"` must bind an installed `typeId`. Only one
creation contribution per type is allowed. Its declared operations also have
`scope: "create"` and `permission: "admin"`. The core binds the type from the
manifest and sends `CreationContext {typeId, params}`; there is no server identity
or filesystem context before creation. Do not reuse server-scoped operations.

The parent fetches the allowlisted HTML with its authenticated API client and
renders `srcdoc` in an opaque `allow-scripts` iframe. The init message identifies
plugin ID/version, contribution ID, server ID and optional recipe pin. No token, cookie or consent
receipt is passed to the screen. Inline scripts/styles are supported; direct
network calls, form submission and same-origin access are blocked by CSP.

After receiving `naviserver:init` and a transferred MessageChannel port, the
screen acknowledges `naviserver:ready`. Option requests use `{id, method,
params}` and receive `{id, result}` or `{id, error}`. A screen can propose
`{type:"naviserver:draft", valid:boolean, data:CreationDraft}`. The draft contains
only name, version, configuration and optional importPath. It is not installation
authority: the parent owns explicit consent, submission, schema validation,
progress and cancellation. Never accept terms inside the iframe. Failed screens
can be replaced explicitly with the standard declarative form.

The bundled Java executable exposes `--write-ui DIRECTORY` for local packaging;
all distribution and development preparation scripts must emit its declared
`create.html` alongside the executable. Do not distribute a manifest referencing
an absent resource.
