package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envLineRegex = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// EnvFileConfig represents a single .env file configuration.
//
// SopsSource is declaration metadata only. TASK-245 §2-1 freezes it as invisible
// to the load path: LoadEnvFile never reads it, never decrypts anything, and
// never changes what Required means. It exists so `dva config env` can prove
// which encrypted file produces this plaintext target without a second source of
// truth alongside env_file.
type EnvFileConfig struct {
	Path       string
	Required   bool
	SopsSource string
}

// Encrypted reports whether this entry names an encrypted source, which is the
// one property that makes it a candidate for a bridge write.
func (e EnvFileConfig) Encrypted() bool { return e.SopsSource != "" }

// LoadEnvFile loads environment variables from .env file(s).
// The config can be: string, []any, or map with files/required keys.
//
// It is the error-returning adapter over ApplyEnvFiles, kept for callers that
// only need "did the whole declaration set load". Like ApplyEnvFiles it is
// atomic: on failure env is unchanged, so no caller can observe values merged
// before a later file failed. Callers that must distinguish missing from
// unreadable from malformed, or that must report every failure rather than the
// first, need the report from ApplyEnvFiles instead.
func LoadEnvFile(envFileConfig any, basePath string, env *Environment) error {
	return ApplyEnvFiles(envFileConfig, basePath, env).Err()
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
				source, _ := it["sops_source"].(string)
				result = append(result, EnvFileConfig{Path: path, Required: required, SopsSource: source})
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

// parseEnvFileStrict parses a dotenv stream and rejects any non-blank,
// non-comment line that is not an assignment.
//
// The returned line number is the discriminator the caller switches on: a
// non-zero line means "line N is not valid dotenv" (malformed), while a zero
// line with a non-nil error means the scanner itself failed (a read fault).
// Before TASK-248 an unrecognized line was silently skipped, which let a typo
// like `PORT 8080` reach the runtime as a missing variable instead of an error.
func parseEnvFileStrict(f *os.File) (map[string]string, int, error) {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := envLineRegex.FindStringSubmatch(line)
		if matches == nil {
			return nil, lineNo, fmt.Errorf("invalid dotenv syntax at line %d", lineNo)
		}

		key := matches[1]
		value := strings.TrimSpace(matches[2])
		// Match dotenv/Compose behavior for unquoted inline comments. This is
		// important for shared .env.example files where values are documented
		// on the same line, e.g. PORT=14011 # Temporal PostgreSQL.
		if len(value) == 0 || (value[0] != '\'' && value[0] != '"') {
			if comment := strings.Index(value, " #"); comment >= 0 {
				value = strings.TrimSpace(value[:comment])
			}
		}
		value = unquoteEnvValue(value)
		vars[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	return vars, 0, nil
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
	for range maxIterations {
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

// ValidateDotenvStream reports how many assignments a dotenv stream holds and,
// on a syntax failure, which line broke.
//
// It returns a count and a line number and nothing else, by design. The bridge
// validates decrypted plaintext before replacing a target (TASK-245 §7-4), and a
// validator that handed back the parsed map would put every secret key and value
// into the caller's memory for no purpose the caller has. parseEnvFileStrict's
// map is discarded here, at the package boundary, rather than trusted not to
// leak on the far side of it.
func ValidateDotenvStream(f *os.File) (count int, line int, err error) {
	vars, line, err := parseEnvFileStrict(f)
	if err != nil {
		return 0, line, err
	}
	return len(vars), 0, nil
}
