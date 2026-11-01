// Package pluginsdk defines the versioned, language-neutral NaviServer plugin contract.
package pluginsdk

import (
	"bytes"
	"encoding/json"
)

const ProtocolMajor = 1
const SchemaVersion = 1

type Manifest struct {
	SchemaVersion    int                   `json:"schemaVersion"`
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Version          string                `json:"version"`
	NaviServerRange  string                `json:"naviserverRange"`
	ProtocolMajor    int                   `json:"protocolMajor"`
	Platforms        []Platform            `json:"platforms"`
	EntryPoint       string                `json:"entryPoint"`
	Types            []ServerType          `json:"types"`
	ProviderBindings []TypeBinding         `json:"providerBindings,omitempty"`
	Operations       []OperationDescriptor `json:"operations,omitempty"`
	UI               []UIContribution      `json:"ui,omitempty"`
}
type Platform struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Distribution string `json:"distribution,omitempty"`
	MinVersion   string `json:"minVersion,omitempty"`
}
type ServerType struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Capabilities  Capabilities `json:"capabilities"`
	Configuration Schema       `json:"configuration"`
}

// TypeBinding lets a provider package contribute optional, non-execution behavior
// to a recipe-owned type when both sides declare compatible versions/capabilities.
type TypeBinding struct {
	TypeID             string       `json:"typeId"`
	RecipeVersionRange string       `json:"recipeVersionRange"`
	Capabilities       Capabilities `json:"capabilities"`
}
type Capabilities struct {
	Console bool               `json:"console"`
	Files   bool               `json:"files"`
	Backups bool               `json:"backups"`
	Players bool               `json:"players"`
	Addons  bool               `json:"addons"`
	Metrics []MetricDescriptor `json:"metrics,omitempty"`
}
type Schema struct {
	Type        string            `json:"type"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	Enum        []any             `json:"enum,omitempty"`
	Default     any               `json:"default,omitempty"`
	Minimum     *float64          `json:"minimum,omitempty"`
	Maximum     *float64          `json:"maximum,omitempty"`
	MinLength   *int              `json:"minLength,omitempty"`
	MaxLength   *int              `json:"maxLength,omitempty"`
	Secret      bool              `json:"secret,omitempty"`
}
type OperationDescriptor struct {
	Method     string `json:"method"`
	Scope      string `json:"scope"`
	Permission string `json:"permission"`
}
type UIContribution struct {
	Resources            []string `json:"resources,omitempty"`
	Operations           []string `json:"operations,omitempty"`
	RequiresCapabilities []string `json:"requiresCapabilities,omitempty"`
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Scope                string   `json:"scope"`
	Path                 string   `json:"path"`
	TypeID               string   `json:"typeId,omitempty"`
}
type Endpoint struct {
	PortEnd  int    `json:"portEnd,omitempty"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port"`
}
type ServerContext struct {
	Status        string           `json:"status"`
	ServerID      string           `json:"serverId"`
	TypeID        string           `json:"typeId"`
	Directory     string           `json:"directory"`
	RuntimesPath  string           `json:"runtimesPath,omitempty"`
	Version       string           `json:"version,omitempty"`
	Loader        string           `json:"loader,omitempty"`
	JavaVersion   int              `json:"javaVersion,omitempty"`
	RAM           int              `json:"ram,omitempty"`
	CustomArgs    string           `json:"customArgs,omitempty"`
	Port          int              `json:"port,omitempty"`
	Configuration map[string]any   `json:"configuration"`
	Endpoints     []Endpoint       `json:"endpoints,omitempty"`
	OperationID   string           `json:"operationId,omitempty"`
	Consent       []ConsentReceipt `json:"consent,omitempty"`
}
type ExecutionPlan struct {
	Preparation  []Action          `json:"preparation,omitempty"`
	Executable   string            `json:"executable"`
	Args         []string          `json:"args"`
	Directory    string            `json:"directory"`
	Env          map[string]string `json:"env,omitempty"`
	Endpoints    []Endpoint        `json:"endpoints"`
	Stop         StopPlan          `json:"stop"`
	ReadyPattern string            `json:"readyPattern,omitempty"`
	Container    *ContainerPlan    `json:"container,omitempty"`
}

// ContainerPlan describes a core-owned rootless container execution. The host
// data source and security policy are deliberately not provider-controlled.
type ContainerPlan struct {
	Image       string                  `json:"image"`
	ImageDigest string                  `json:"imageDigest"`
	DataMount   string                  `json:"dataMount"`
	UID         uint32                  `json:"uid"`
	GID         uint32                  `json:"gid"`
	Installer   *ContainerCommand       `json:"installer,omitempty"`
	Runtime     ContainerCommand        `json:"runtime"`
	Endpoints   []ContainerEndpoint     `json:"endpoints"`
	Limits      ContainerResourceLimits `json:"limits"`
}

type ContainerCommand struct {
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
}

type ContainerEndpoint struct {
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
}

type ContainerResourceLimits struct {
	CPUCores    float64 `json:"cpuCores"`
	MemoryBytes uint64  `json:"memoryBytes"`
	PIDs        uint32  `json:"pids"`
}

func (p *ContainerPlan) UnmarshalJSON(data []byte) error {
	type containerPlan ContainerPlan
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var decoded containerPlan
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*p = ContainerPlan(decoded)
	return nil
}

type StopPlan struct {
	Command        string `json:"command,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}
type BackupPolicy struct {
	Includes []string `json:"includes"`
	Excludes []string `json:"excludes,omitempty"`
	Before   []Action `json:"before,omitempty"`
	After    []Action `json:"after,omitempty"`
}
type Action struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}
type MetricDescriptor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Unit string `json:"unit,omitempty"`
}
type Metrics struct {
	Players    *PlayerStats   `json:"players,omitempty"`
	Additional map[string]any `json:"additional,omitempty"`
}
type PlayerStats struct {
	Online  int      `json:"online"`
	Max     int      `json:"max"`
	Players []Player `json:"players"`
}
type Player struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type ConsentRequirement struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Version string `json:"version"`
}
type ConsentReceipt struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	AcceptedAt string `json:"acceptedAt"`
	UserID     string `json:"userId"`
}
type Progress struct {
	OperationID string  `json:"operationId"`
	Message     string  `json:"message"`
	Percent     float64 `json:"percent"`
}
type ProvisionResult struct {
	Version       string         `json:"version"`
	Configuration map[string]any `json:"configuration"`
}

// RestoreContext prepares an owned staging directory only. TargetConfiguration is the
// destination server configuration whose networking must remain unchanged.
type RestoreContext struct {
	ServerContext
	TargetConfiguration map[string]any `json:"targetConfiguration"`
}

// CreationContext identifies an installed server type before an instance exists.
// The core supplies TypeID; screens can supply only Params, never instance paths,
// credentials, consent receipts or an existing server identity.
type CreationContext struct {
	TypeID string          `json:"typeId"`
	Params json.RawMessage `json:"params"`
}

// CreationDraft is an untrusted screen proposal, validated by the core before
// submission. Identity and consent are deliberately absent from this contract.
type CreationDraft struct {
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Configuration map[string]any `json:"configuration"`
	ImportPath    string         `json:"importPath,omitempty"`
}
