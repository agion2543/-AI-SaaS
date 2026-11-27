package config

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type productionCompose struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Command     []string       `yaml:"command"`
	Environment map[string]any `yaml:"environment"`
	DependsOn   map[string]struct {
		Condition string `yaml:"condition"`
	} `yaml:"depends_on"`
	Healthcheck struct {
		Test []string `yaml:"test"`
	} `yaml:"healthcheck"`
}

// UnmarshalYAML accepts the Compose spellings used across the repo: command
// as a single string or a list, and depends_on as a list of service names or
// a map of service to condition. The gate assertions only care about the
// schema-migrate service command and the backend's dependency condition.
func (s *composeService) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Command     yaml.Node      `yaml:"command"`
		Environment map[string]any `yaml:"environment"`
		DependsOn   yaml.Node      `yaml:"depends_on"`
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	s.Environment = raw.Environment
	s.Healthcheck = raw.Healthcheck

	if raw.Command.Kind != 0 {
		switch raw.Command.Kind {
		case yaml.ScalarNode:
			var command string
			if err := raw.Command.Decode(&command); err != nil {
				return err
			}
			s.Command = []string{command}
		case yaml.SequenceNode:
			if err := raw.Command.Decode(&s.Command); err != nil {
				return err
			}
		default:
			return &yaml.TypeError{Errors: []string{"command must be a string or a list of strings"}}
		}
	}

	s.DependsOn = map[string]struct {
		Condition string `yaml:"condition"`
	}{}
	if raw.DependsOn.Kind != 0 {
		switch raw.DependsOn.Kind {
		case yaml.MappingNode:
			if err := raw.DependsOn.Decode(&s.DependsOn); err != nil {
				return err
			}
		case yaml.SequenceNode:
			var names []string
			if err := raw.DependsOn.Decode(&names); err != nil {
				return err
			}
			for _, name := range names {
				s.DependsOn[name] = struct {
					Condition string `yaml:"condition"`
				}{}
			}
		default:
			return &yaml.TypeError{Errors: []string{"depends_on must be a list or a map"}}
		}
	}
	return nil
}

func readProductionCompose(t *testing.T, name string) productionCompose {
	t.Helper()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate the config test source file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	data, err := os.ReadFile(filepath.Join(repoRoot, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	var compose productionCompose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(compose.Services) == 0 {
		t.Fatalf("%s has no Compose services", name)
	}
	return compose
}

func TestProductionComposeUsesVersionedMigrationGate(t *testing.T) {
	for _, name := range []string{
		"docker-compose.yml",
		"docker-compose.prod.yml",
		"docker-compose.new-install.yml",
		"docker-compose.managed.yml",
	} {
		t.Run(name, func(t *testing.T) {
			compose := readProductionCompose(t, name)
			migrator, ok := compose.Services["schema-migrate"]
			if !ok || !containsString(migrator.Command, "/app/schema-migrate") {
				t.Fatalf("%s must run the versioned /app/schema-migrate command", name)
			}

			backend, ok := compose.Services["backend"]
			if !ok {
				t.Fatalf("%s has no backend service", name)
			}
			if !isFalse(backend.Environment["DB_AUTO_MIGRATE"]) {
				t.Fatalf("%s backend must explicitly disable app-side DB_AUTO_MIGRATE", name)
			}
			if name == "docker-compose.new-install.yml" || name == "docker-compose.yml" {
				if !isTrue(backend.Environment["INITIAL_DATA_SEED_ENABLED"]) {
					t.Fatalf("%s backend must explicitly enable initial data seeding", name)
				}
			} else if !isFalse(backend.Environment["INITIAL_DATA_SEED_ENABLED"]) {
				t.Fatalf("%s backend must explicitly disable initial data seeding", name)
			}

			dependency, ok := backend.DependsOn["schema-migrate"]
			if !ok || dependency.Condition != "service_completed_successfully" {
				t.Fatalf("%s backend must depend on schema-migrate completing successfully; got %#v", name, backend.DependsOn)
			}
		})
	}
}

func TestMySQLProductionComposeRequiresExplicitDatabasePasswords(t *testing.T) {
	for _, name := range []string{
		"docker-compose.prod.yml",
		"docker-compose.new-install.yml",
	} {
		t.Run(name, func(t *testing.T) {
			compose := readProductionCompose(t, name)
			mysql, ok := compose.Services["mysql"]
			if !ok {
				t.Fatalf("%s must define its local MySQL service", name)
			}

			requireEnvironmentReference(t, name, mysql.Environment, "MYSQL_ROOT_PASSWORD", "MYSQL_ROOT_PASSWORD")
			requireEnvironmentReference(t, name, mysql.Environment, "MYSQL_PASSWORD", "MYSQL_APP_PASSWORD")

			healthcheck := strings.Join(mysql.Healthcheck.Test, " ")
			if !strings.Contains(healthcheck, "-p${MYSQL_ROOT_PASSWORD:?") {
				t.Fatalf("%s MySQL healthcheck must use the required MYSQL_ROOT_PASSWORD variable without a default", name)
			}
		})
	}
}

var requiredComposeEnvironmentValue = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*):\?([^}]*)\}$`)

func requireEnvironmentReference(t *testing.T, file string, environment map[string]any, key, variable string) {
	t.Helper()
	value, ok := environment[key]
	if !ok {
		t.Fatalf("%s MySQL environment is missing %s", file, key)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s MySQL %s must be a required environment-variable expression, not %T", file, key, value)
	}
	match := requiredComposeEnvironmentValue.FindStringSubmatch(text)
	if len(match) != 3 || match[1] != variable || strings.TrimSpace(match[2]) == "" {
		t.Fatalf("%s MySQL %s must require an explicitly supplied %s and must not contain a public/default password; got %q", file, key, variable, text)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func isFalse(value any) bool {
	switch value := value.(type) {
	case bool:
		return !value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "false")
	default:
		return false
	}
}

func isTrue(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}
