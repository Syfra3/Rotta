package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	openCodeGlobalConfigSource   = "XDG global configuration"
	openCodeOverrideConfigSource = "OPENCODE_CONFIG override"
	openCodeProjectConfigSource  = "documented project configuration"
)

// OpenCodeConfigResolution reports the documented layer selected for a
// selected-host OpenCode configuration write.
type OpenCodeConfigResolution struct {
	Path       string
	Source     string
	Format     string
	Precedence []string
}

type openCodeConfigDocument struct {
	config     map[string]interface{}
	resolution OpenCodeConfigResolution
	comments   []string
}

func resolveOpenCodeConfig(opts Options, home string) (OpenCodeConfigResolution, error) {
	precedence := []string{openCodeGlobalConfigSource, openCodeOverrideConfigSource, openCodeProjectConfigSource}
	global := filepath.Join(openCodeConfigHome(home), "opencode", "opencode.json")
	resolution := newOpenCodeConfigResolution(global, openCodeGlobalConfigSource, precedence)

	if override := os.Getenv("OPENCODE_CONFIG"); override != "" {
		resolution = newOpenCodeConfigResolution(override, openCodeOverrideConfigSource, precedence)
	}

	project := resolveProjectPath(opts.ProjectPath, home)
	projectPath, err := existingOpenCodeProjectConfig(project)
	if err != nil {
		return OpenCodeConfigResolution{}, err
	}
	if projectPath != "" {
		resolution = newOpenCodeConfigResolution(projectPath, openCodeProjectConfigSource, precedence)
	}
	return resolution, nil
}

func openCodeConfigHome(home string) string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return configHome
	}
	return filepath.Join(home, ".config")
}

func existingOpenCodeProjectConfig(project string) (string, error) {
	var selected string
	for _, path := range []string{filepath.Join(project, "opencode.json"), filepath.Join(project, "opencode.jsonc")} {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cannot inspect project OpenCode config %s: %w", path, err)
		}
		if selected != "" {
			return "", fmt.Errorf("cannot select one documented project OpenCode config")
		}
		selected = path
	}
	return selected, nil
}

func newOpenCodeConfigResolution(path, source string, precedence []string) OpenCodeConfigResolution {
	format := "JSON"
	if strings.EqualFold(filepath.Ext(path), ".jsonc") {
		format = "JSONC"
	}
	return OpenCodeConfigResolution{Path: path, Source: source, Format: format, Precedence: append([]string(nil), precedence...)}
}

func readResolvedOpenCodeConfig(resolution OpenCodeConfigResolution) (openCodeConfigDocument, error) {
	document := openCodeConfigDocument{config: map[string]interface{}{}, resolution: resolution}
	data, err := readPrivateFile(resolution.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return document, nil
		}
		return openCodeConfigDocument{}, fmt.Errorf("cannot read OpenCode config: %w", err)
	}
	if resolution.Format == "JSONC" {
		document.comments = jsoncComments(data)
		data = []byte(stripJSONCTokens(string(data)))
	}
	if err := json.Unmarshal(data, &document.config); err != nil {
		return openCodeConfigDocument{}, fmt.Errorf("cannot parse %s: %w", resolution.Path, err)
	}
	return document, nil
}

// validateSelectedOpenCodeConfiguration performs documented OpenCode
// configuration checks before a selected OpenCode installation can clean hosts.
func validateSelectedOpenCodeConfiguration(opts Options, home string) error {
	if !targetsOpenCode(opts.Target) {
		return nil
	}
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return fmt.Errorf("effective-config resolution blocked: %w", err)
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return fmt.Errorf("schema validation blocked: %w", err)
	}
	if err := validateOpenCodeConfigurationShape(document.config); err != nil {
		return fmt.Errorf("schema validation blocked: %w", err)
	}
	return nil
}

func targetsOpenCode(target string) bool {
	return target == "opencode" || target == "both" || target == "all"
}

// validateOpenCodeConfigurationShape checks the configuration sections this
// installer reads and writes using OpenCode's documented config schema shape.
// See https://opencode.ai/config.json: agent, compaction, and tool_output are
// configuration objects, not scalar readiness flags.
func validateOpenCodeConfigurationShape(config map[string]interface{}) error {
	for _, key := range []string{"agent", "compaction", "tool_output"} {
		value, ok := config[key]
		if !ok {
			continue
		}
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("documented OpenCode %s configuration must be an object", key)
		}
	}
	return nil
}

func writeResolvedOpenCodeConfig(document openCodeConfigDocument) error {
	if err := os.MkdirAll(filepath.Dir(document.resolution.Path), 0o750); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}
	out, err := json.MarshalIndent(document.config, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal OpenCode config: %w", err)
	}
	if document.resolution.Format == "JSONC" && len(document.comments) != 0 {
		out = append([]byte(strings.Join(document.comments, "\n")+"\n"), out...)
	}
	return writePrivateFile(document.resolution.Path, out, 0o600)
}

func stripJSONCTokens(input string) string {
	withoutComments := stripJSONCComments(input)
	var output strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(withoutComments); i++ {
		current := withoutComments[i]
		if inString {
			output.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			output.WriteByte(current)
			continue
		}
		if current == ',' {
			next := i + 1
			for next < len(withoutComments) && strings.ContainsRune(" \t\r\n", rune(withoutComments[next])) {
				next++
			}
			if next < len(withoutComments) && (withoutComments[next] == '}' || withoutComments[next] == ']') {
				continue
			}
		}
		output.WriteByte(current)
	}
	return output.String()
}

func stripJSONCComments(input string) string {
	var output strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		current := input[i]
		if inString {
			output.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			output.WriteByte(current)
			continue
		}
		if current == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i+1 < len(input) && input[i+1] != '\n' {
				i++
			}
			continue
		}
		if current == '/' && i+1 < len(input) && input[i+1] == '*' {
			i += 2
			for i < len(input) && (input[i] != '*' || i+1 == len(input) || input[i+1] != '/') {
				if input[i] == '\n' {
					output.WriteByte('\n')
				}
				i++
			}
			i++
			continue
		}
		output.WriteByte(current)
	}
	return output.String()
}

func jsoncComments(data []byte) []string {
	var comments []string
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		current := data[i]
		if inString {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			continue
		}
		if current != '/' || i+1 == len(data) {
			continue
		}
		start := i
		switch data[i+1] {
		case '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			comments = append(comments, string(data[start:i]))
		case '*':
			i += 2
			for i+1 < len(data) && (data[i] != '*' || data[i+1] != '/') {
				i++
			}
			if i+1 < len(data) {
				i++
			}
			comments = append(comments, string(data[start:i+1]))
		}
	}
	return comments
}
