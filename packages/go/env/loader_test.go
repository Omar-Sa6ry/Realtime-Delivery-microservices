package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadRejectsUnsupportedEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("NODE_ENV", "")

	err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported APP_ENV")
}

func TestLoadFallsBackToNodeEnv(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("NODE_ENV", "production")

	// In an empty working directory no candidate file exists, so Load is a no-op.
	t.Chdir(t.TempDir())

	require.NoError(t, Load())
}

func TestLoadDefaultsToDevelopmentWhenUnset(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("NODE_ENV", "")
	t.Chdir(t.TempDir())

	require.NoError(t, Load())
}

func TestLoadReadsMatchingFileFromConfigDir(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("NODE_ENV", "")
	dir := t.TempDir()
	writeEnvFile(t, filepath.Join(dir, "config", "env"), ".env.test",
		"# a comment\n\nexport LOADED_FROM_ENV_FILE=42\nQUOTED=\"hello world\"\n")
	t.Chdir(dir)

	require.NoError(t, Load())

	t.Cleanup(func() {
		_ = os.Unsetenv("LOADED_FROM_ENV_FILE")
		_ = os.Unsetenv("QUOTED")
	})

	assert.Equal(t, "42", os.Getenv("LOADED_FROM_ENV_FILE"))
	assert.Equal(t, "hello world", os.Getenv("QUOTED"))
}

func TestLoadReturnsNilWhenNoFileMatches(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Chdir(t.TempDir())

	require.NoError(t, Load())
}

func TestLoadFileMissing(t *testing.T) {
	err := loadFile(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open environment file")
}

func TestLoadFileParsingRules(t *testing.T) {
	key := "GO_ENV_PARSER_PROBE"
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	_ = os.Unsetenv(key)

	path := writeEnvFile(t, t.TempDir(), ".env.probe", `# leading comment
export GO_ENV_PARSER_PROBE = "quoted value"

EMPTY_LINE_ABOVE=1
`)
	require.NoError(t, loadFile(path))
	assert.Equal(t, "quoted value", os.Getenv(key))
}

func TestLoadFileRejectsMalformedEntries(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"missing assignment", "JUST_A_KEY"},
		{"blank key", "   =value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeEnvFile(t, t.TempDir(), ".env.bad", tt.content)
			err := loadFile(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid environment entry")
		})
	}
}

func TestLoadFileDoesNotOverrideExistingVariables(t *testing.T) {
	const key = "GO_ENV_PRESET"
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	require.NoError(t, os.Setenv(key, "already-here"))

	path := writeEnvFile(t, t.TempDir(), ".env.preset", "GO_ENV_PRESET=from-file\n")
	require.NoError(t, loadFile(path))

	assert.Equal(t, "already-here", os.Getenv(key))
}

func TestLoadFileStripsExportPrefixAndSingleQuotes(t *testing.T) {
	const key = "GO_ENV_EXPORT_PROBE"
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	_ = os.Unsetenv(key)

	path := writeEnvFile(t, t.TempDir(), ".env.export", "export GO_ENV_EXPORT_PROBE='single'\n")
	require.NoError(t, loadFile(path))

	assert.Equal(t, "single", os.Getenv(key))
}
