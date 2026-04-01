package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envLineRegex = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// EnvFileConfig represents a single .env file configuration.
type EnvFileConfig struct {
	Path     string
	Required bool
}

// LoadEnvFile loads environment variables from .env file(s).
// The config can be: string, []any, or map with files/priority/interpolate keys.
func LoadEnvFile(envFileConfig any, basePath string, env *Environment) error {
	files := normalizeEnvFileConfig(envFileConfig)

	for _, f := range files {
		path := f.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(basePath, path)
		}

		data, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				if f.Required {
					return fmt.Errorf("required environment file not found: %s", path)
				}
				continue // optional, skip
			}
			return fmt.Errorf("reading env file %s: %w", path, err)
		}

		vars, err := parseEnvFile(data)
		_ = data.Close()
		if err != nil {
			return fmt.Errorf("parsing env file %s: %w", path, err)
		}

		// Merge into environment respecting OS priority (env_file < OS env)
		env.MergeVars(vars)
	}

	// Interpolate loaded vars
	interpolateEnvVars(env)

	return nil
}

func normalizeEnvFileConfig(config any) []EnvFileConfig {
	switch v := config.(type) {
	case string:
		return []EnvFileConfig{{Path: v, Required: false}}
	case []any:
		var result []EnvFileConfig
		for _, item := range v {
			switch it := item.(type) {
			case string:
				result = append(result, EnvFileConfig{Path: it, Required: false})
			case map[string]any:
				path, _ := it["path"].(string)
				required, _ := it["required"].(bool)
				result = append(result, EnvFileConfig{Path: path, Required: required})
			}
		}
		return result
	case map[string]any:
		files := v["files"]
		required, _ := v["required"].(bool)
		configs := normalizeEnvFileConfig(files)
		for i := range configs {
			configs[i].Required = configs[i].Required || required
		}
		return configs
	}
	return nil
}

func parseEnvFile(f *os.File) (map[string]string, error) {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := envLineRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		key := matches[1]
		value := strings.TrimSpace(matches[2])
		value = unquoteEnvValue(value)
		vars[key] = value
	}

	return vars, scanner.Err()
}

func unquoteEnvValue(value string) string {
	// Single quotes: no escape sequences
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}

	// Double quotes: process escape sequences
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		value = strings.ReplaceAll(value, `\\`, "\x00")
		value = strings.ReplaceAll(value, `\n`, "\n")
		value = strings.ReplaceAll(value, `\t`, "\t")
		value = strings.ReplaceAll(value, `\r`, "\r")
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, "\x00", `\`)
		return value
	}

	return value
}

func interpolateEnvVars(env *Environment) {
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		changed := false
		for k, v := range env.Vars {
			newV := env.Interpolate(v)
			if newV != v {
				env.Vars[k] = newV
				changed = true
			}
		}
		if !changed {
			break
		}
	}
}
