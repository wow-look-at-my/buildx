package gitutil

import (
	"os"
	"testing"

	"github.com/docker/buildx/util/osutil"
	"github.com/stretchr/testify/assert"
)

func TestSanitizePathWindows(t *testing.T) {
	expected := "C:\\Users\\foobar"
	if isGitBash() {
		expected = "C:/Users/foobar"
	}
	assert.Equal(t, expected, osutil.SanitizePath("C:/Users/foobar"))
}

func isGitBash() bool {
	// The MSYSTEM environment variable is used in MSYS2 environments, including
	// Git Bash, to select the active environment.
	if _, ok := os.LookupEnv("MSYSTEM"); ok {
		return true
	}
	return false
}
