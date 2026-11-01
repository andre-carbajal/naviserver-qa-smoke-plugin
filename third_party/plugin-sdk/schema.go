package pluginsdk

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func parseSemver(version string) []string {
	parts := semver.FindStringSubmatch(version)
	if parts == nil {
		return nil
	}
	metadata := ""
	if i := strings.IndexByte(version, '+'); i >= 0 {
		metadata = version[i+1:]
	}
	if !validIdentifiers(parts[4], true) || !validIdentifiers(metadata, false) {
		return nil
	}
	return parts
}

func validIdentifiers(section string, prerelease bool) bool {
	if section == "" {
		return true
	}
	for _, id := range strings.Split(section, ".") {
		if id == "" || prerelease && numericIdentifier(id) && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func compareNumericIdentifiers(a, b string) int {
	if len(a) < len(b) || len(a) == len(b) && a < b {
		return -1
	}
	if len(a) > len(b) || a > b {
		return 1
	}
	return 0
}

func compareVersions(a, b string) (int, error) {
	am, bm := parseSemver(a), parseSemver(b)
	if am == nil || bm == nil {
		return 0, fmt.Errorf("invalid semantic version")
	}
	for i := 1; i <= 3; i++ {
		if cmp := compareNumericIdentifiers(am[i], bm[i]); cmp != 0 {
			return cmp, nil
		}
	}
	if am[4] == bm[4] {
		return 0, nil
	}
	if am[4] == "" {
		return 1, nil
	}
	if bm[4] == "" {
		return -1, nil
	}
	as, bs := strings.Split(am[4], "."), strings.Split(bm[4], ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, bn := numericIdentifier(as[i]), numericIdentifier(bs[i])
		if an && bn {
			return compareNumericIdentifiers(as[i], bs[i]), nil
		}
		if an {
			return -1, nil
		}
		if bn {
			return 1, nil
		}
		if as[i] < bs[i] {
			return -1, nil
		}
		return 1, nil
	}
	if len(as) < len(bs) {
		return -1, nil
	}
	if len(as) > len(bs) {
		return 1, nil
	}
	return 0, nil
}

// VersionMatches accepts exact, wildcard and whitespace-separated comparators (AND), with || alternatives.
func VersionMatches(version, constraint string) bool {
	if parseSemver(version) == nil || ValidateVersionConstraint(constraint) != nil {
		return false
	}
	for _, alt := range strings.Split(constraint, "||") {
		valid := true
		fields := strings.Fields(strings.TrimSpace(alt))
		if len(fields) == 0 {
			continue
		}
		for _, term := range fields {
			if term == "*" {
				continue
			}
			op := "="
			target := term
			for _, prefix := range []string{">=", "<=", ">", "<", "="} {
				if strings.HasPrefix(term, prefix) {
					op = prefix
					target = strings.TrimPrefix(term, prefix)
					break
				}
			}
			cmp, err := compareVersions(version, target)
			if err != nil {
				valid = false
				break
			}
			ok := cmp == 0
			switch op {
			case ">=":
				ok = cmp >= 0
			case "<=":
				ok = cmp <= 0
			case ">":
				ok = cmp > 0
			case "<":
				ok = cmp < 0
			}
			valid = valid && ok
		}
		if valid {
			return true
		}
	}
	return false
}

// ValidateVersionConstraint accepts the same bounded comparator grammar as VersionMatches.
func ValidateVersionConstraint(constraint string) error {
	if len(constraint) == 0 || len(constraint) > 256 {
		return fmt.Errorf("invalid version constraint")
	}
	for _, alternative := range strings.Split(constraint, "||") {
		fields := strings.Fields(strings.TrimSpace(alternative))
		if len(fields) == 0 || len(fields) > 16 {
			return fmt.Errorf("invalid version constraint")
		}
		for _, term := range fields {
			if term == "*" {
				continue
			}
			target := term
			for _, prefix := range []string{">=", "<=", ">", "<", "="} {
				if strings.HasPrefix(term, prefix) {
					target = strings.TrimPrefix(term, prefix)
					break
				}
			}
			if parseSemver(target) == nil {
				return fmt.Errorf("invalid version constraint")
			}
		}
	}
	return nil
}
func SupportsPlatform(m Manifest, os, arch string) bool {
	for _, p := range m.Platforms {
		if p.OS == os && p.Arch == arch {
			return true
		}
	}
	return false
}
func ValidateManifest(m Manifest) error {
	if m.SchemaVersion != SchemaVersion || m.ProtocolMajor != ProtocolMajor {
		return fmt.Errorf("unsupported manifest/protocol version")
	}
	if !identifier.MatchString(m.ID) || m.Name == "" || parseSemver(m.Version) == nil {
		return fmt.Errorf("invalid plugin identity/version")
	}
	if m.NaviServerRange == "" {
		return fmt.Errorf("missing daemon compatibility range")
	}
	if m.EntryPoint == "" || filepath.IsAbs(m.EntryPoint) || strings.Contains(m.EntryPoint, "\\") || strings.Contains(m.EntryPoint, ":") || filepath.Clean(m.EntryPoint) == ".." || strings.HasPrefix(filepath.Clean(m.EntryPoint), "../") {
		return fmt.Errorf("invalid entrypoint")
	}
	if len(m.Platforms) == 0 || (len(m.Types) == 0 && len(m.ProviderBindings) == 0) {
		return fmt.Errorf("missing platforms or server types/provider bindings")
	}
	seen := map[string]bool{}
	for _, t := range m.Types {
		if !identifier.MatchString(t.ID) || t.Name == "" || seen[t.ID] {
			return fmt.Errorf("invalid/duplicate server type")
		}
		seen[t.ID] = true
		if err := ValidateSchema(t.Configuration); err != nil {
			return fmt.Errorf("type %s: %w", t.ID, err)
		}
		if err := ValidateMetricDescriptors(t.Capabilities.Metrics); err != nil {
			return fmt.Errorf("type %s: %w", t.ID, err)
		}
	}
	bindings := map[string]TypeBinding{}
	for _, binding := range m.ProviderBindings {
		if !strings.HasPrefix(binding.TypeID, "recipe.") || !identifier.MatchString(binding.TypeID) || len(binding.TypeID) <= len("recipe.") || ValidateVersionConstraint(binding.RecipeVersionRange) != nil || binding.Capabilities.Console || binding.Capabilities.Files || binding.Capabilities.Backups {
			return fmt.Errorf("invalid recipe provider binding")
		}
		if _, duplicate := bindings[binding.TypeID]; duplicate {
			return fmt.Errorf("duplicate recipe provider binding")
		}
		if err := ValidateMetricDescriptors(binding.Capabilities.Metrics); err != nil {
			return err
		}
		bindings[binding.TypeID] = binding
	}

	operations := map[string]OperationDescriptor{}
	for _, op := range m.Operations {
		if !identifier.MatchString(op.Method) {
			return fmt.Errorf("invalid operation method")
		}
		if _, duplicate := operations[op.Method]; duplicate {
			return fmt.Errorf("duplicate operation method")
		}
		if op.Scope == "global" || op.Scope == "create" {
			if op.Permission != "admin" {
				return fmt.Errorf("global operations require admin permission")
			}
		} else if op.Scope == "server" {
			if op.Permission != "console" && op.Permission != "power" {
				return fmt.Errorf("invalid server operation permission")
			}
		} else {
			return fmt.Errorf("invalid operation scope")
		}
		operations[op.Method] = op
	}
	contributions := map[string]bool{}
	creationTypes := map[string]bool{}
	for _, ui := range m.UI {
		if !identifier.MatchString(ui.ID) || ui.Title == "" || contributions[ui.ID] {
			return fmt.Errorf("invalid/duplicate contribution")
		}
		contributions[ui.ID] = true
		if ui.Scope != "global" && ui.Scope != "server" && ui.Scope != "create" {
			return fmt.Errorf("invalid contribution scope")
		}
		if ui.TypeID != "" {
			binding, bound := bindings[ui.TypeID]
			if (ui.Scope != "server" && ui.Scope != "create") || (!seen[ui.TypeID] && !bound) {
				return fmt.Errorf("unknown contribution server type")
			}
			var capabilities Capabilities
			if bound {
				capabilities = binding.Capabilities
			} else {
				for _, serverType := range m.Types {
					if serverType.ID == ui.TypeID {
						capabilities = serverType.Capabilities
						break
					}
				}
			}
			required := map[string]bool{}
			for _, capability := range ui.RequiresCapabilities {
				if required[capability] || !SupportsCapability(capabilities, capability) {
					return fmt.Errorf("invalid or unavailable UI capability requirement")
				}
				required[capability] = true
			}
		} else if len(ui.RequiresCapabilities) != 0 {
			return fmt.Errorf("capability-bound contribution requires a server type")
		}
		if ui.Scope == "create" && ui.TypeID == "" {
			return fmt.Errorf("creation contribution requires server type")
		}
		if ui.Scope == "create" {
			if creationTypes[ui.TypeID] {
				return fmt.Errorf("duplicate creation contribution for server type")
			}
			creationTypes[ui.TypeID] = true
		}
		resources := append([]string{ui.Path}, ui.Resources...)
		for _, resource := range resources {
			if !safeRelativePath(resource) || resource == m.EntryPoint || resource == "manifest.json" {
				return fmt.Errorf("invalid/private UI resource")
			}
			for _, part := range strings.Split(resource, "/") {
				if strings.HasPrefix(part, ".") {
					return fmt.Errorf("private UI resource forbidden")
				}
			}
		}
		listed := map[string]bool{}
		for _, method := range ui.Operations {
			op, ok := operations[method]
			if !ok || op.Scope != ui.Scope || listed[method] {
				return fmt.Errorf("undeclared/duplicate/mismatched contribution operation")
			}
			listed[method] = true
		}
	}
	return nil
}
func safeRelativePath(p string) bool {
	return p != "" && p != "." && !filepath.IsAbs(p) && !strings.ContainsAny(p, "\\:") && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func ValidateSchema(s Schema) error {
	switch s.Type {
	case "object":
		for _, p := range s.Properties {
			if err := ValidateSchema(p); err != nil {
				return err
			}
		}
		for _, key := range s.Required {
			if _, ok := s.Properties[key]; !ok {
				return fmt.Errorf("required field %s not declared", key)
			}
		}
	case "array":
		if s.Items == nil {
			return fmt.Errorf("array items required")
		}
		return ValidateSchema(*s.Items)
	case "string", "number", "integer", "boolean":
	default:
		return fmt.Errorf("unsupported schema type %q", s.Type)
	}
	return nil
}

func ValidateMetricDescriptors(metrics []MetricDescriptor) error {
	if len(metrics) > 64 {
		return fmt.Errorf("too many metric descriptors")
	}
	seen := map[string]bool{}
	for _, metric := range metrics {
		if !identifier.MatchString(metric.ID) || seen[metric.ID] || strings.TrimSpace(metric.Name) == "" || len(metric.Name) > 120 || len(metric.Unit) > 32 {
			return fmt.Errorf("invalid or duplicate metric descriptor")
		}
		switch metric.Type {
		case "number", "integer", "string", "boolean":
		default:
			return fmt.Errorf("invalid metric type")
		}
		seen[metric.ID] = true
	}
	return nil
}

func SupportsCapability(capabilities Capabilities, requirement string) bool {
	switch requirement {
	case "players":
		return capabilities.Players
	case "addons":
		return capabilities.Addons
	default:
		metricID, ok := strings.CutPrefix(requirement, "metric:")
		if !ok || !identifier.MatchString(metricID) {
			return false
		}
		for _, metric := range capabilities.Metrics {
			if metric.ID == metricID {
				return true
			}
		}
		return false
	}
}

func clone(v any) any { b, _ := json.Marshal(v); var out any; _ = json.Unmarshal(b, &out); return out }
func ApplyDefaults(s Schema, value map[string]any) map[string]any {
	v, _ := clone(value).(map[string]any)
	if v == nil {
		v = map[string]any{}
	}
	for k, p := range s.Properties {
		x, ok := v[k]
		if !ok && p.Default != nil {
			x = clone(p.Default)
			v[k] = x
		}
		if p.Type == "object" {
			if obj, ok := x.(map[string]any); ok {
				v[k] = ApplyDefaults(p, obj)
			}
		}
	}
	return v
}
func ValidateConfiguration(s Schema, v any) error { return validateValue(s, v, "configuration") }
func validateValue(s Schema, v any, path string) error {
	fail := func() error { return fmt.Errorf("%s: invalid %s value", path, s.Type) }
	switch s.Type {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		for _, key := range s.Required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("%s.%s: required", path, key)
			}
		}
		for k, x := range obj {
			p, ok := s.Properties[k]
			if !ok {
				return fmt.Errorf("%s.%s: unknown field", path, k)
			}
			if e := validateValue(p, x, path+"."+k); e != nil {
				return e
			}
		}
	case "array":
		xs, ok := v.([]any)
		if !ok || s.Items == nil {
			return fail()
		}
		for i, x := range xs {
			if e := validateValue(*s.Items, x, fmt.Sprintf("%s[%d]", path, i)); e != nil {
				return e
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok {
			return fail()
		}
		n := len([]rune(x))
		if s.MinLength != nil && n < *s.MinLength || s.MaxLength != nil && n > *s.MaxLength {
			return fail()
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fail()
		}
	case "number", "integer":
		b, e := json.Marshal(v)
		if e != nil {
			return fail()
		}
		var n float64
		if json.Unmarshal(b, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fail()
		}
		if s.Type == "integer" && n != math.Trunc(n) || s.Minimum != nil && n < *s.Minimum || s.Maximum != nil && n > *s.Maximum {
			return fail()
		}
	default:
		return fail()
	}
	if len(s.Enum) > 0 {
		found := false
		for _, x := range s.Enum {
			if reflect.DeepEqual(clone(x), clone(v)) {
				found = true
			}
		}
		if !found {
			return fail()
		}
	}
	return nil
}
func RedactConfiguration(s Schema, value map[string]any) map[string]any {
	v, _ := redact(s, value).(map[string]any)
	return v
}
func redact(s Schema, v any) any {
	if s.Secret {
		return "[REDACTED]"
	}
	if obj, ok := v.(map[string]any); ok {
		out := map[string]any{}
		for k, x := range obj {
			out[k] = redact(s.Properties[k], x)
		}
		return out
	}
	if arr, ok := v.([]any); ok && s.Items != nil {
		out := make([]any, len(arr))
		for i, x := range arr {
			out[i] = redact(*s.Items, x)
		}
		return out
	}
	return clone(v)
}
