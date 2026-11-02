package bake

import _ "embed"

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCompose(t *testing.T) {
	dt := []byte(testParseComposeYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.Groups))
	require.Equal(t, "default", c.Groups[0].Name)
	sort.Strings(c.Groups[0].Targets)
	require.Equal(t, []string{"db", "webapp", "webapp2"}, c.Groups[0].Targets)

	require.Equal(t, 3, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	require.Equal(t, "db", c.Targets[0].Name)
	require.Equal(t, "db", *c.Targets[0].Context)
	require.Equal(t, []string{"docker.io/tonistiigi/db"}, c.Targets[0].Tags)

	require.Equal(t, "webapp", c.Targets[1].Name)
	require.Equal(t, "dir", *c.Targets[1].Context)
	require.Equal(t, map[string]string{"foo": "bar"}, c.Targets[1].Contexts)
	require.Equal(t, "Dockerfile-alternate", *c.Targets[1].Dockerfile)
	require.Equal(t, 1, len(c.Targets[1].Args))
	require.Equal(t, ptrstr("123"), c.Targets[1].Args["buildno"])
	require.Equal(t, []string{"type=local,src=path/to/cache"}, stringify(c.Targets[1].CacheFrom))
	require.Equal(t, []string{"type=local,dest=path/to/cache"}, stringify(c.Targets[1].CacheTo))
	require.Equal(t, map[string]*string{"myhostv6": ptrstr("::1"), "somehost": ptrstr("162.242.195.82,162.242.195.83")}, c.Targets[1].ExtraHosts)
	require.Equal(t, "none", *c.Targets[1].NetworkMode)
	require.Equal(t, []string{"default", "key=/path/to/key"}, stringify(c.Targets[1].SSH))
	require.Equal(t, []string{
		"id=aws,src=/root/.aws/credentials",
		"id=token,env=ENV_TOKEN",
	}, stringify(c.Targets[1].Secrets))

	require.Equal(t, "webapp2", c.Targets[2].Name)
	require.Equal(t, "dir", *c.Targets[2].Context)
	require.Equal(t, "FROM alpine\n", *c.Targets[2].DockerfileInline)
}

func TestParseComposeEmptyCacheLists(t *testing.T) {
	dt := []byte(testParseComposeEmptyCacheListsYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Len(t, c.Targets, 1)
	require.Empty(t, c.Targets[0].CacheFrom)
	require.Empty(t, c.Targets[0].CacheTo)
}

func TestNoBuildOutOfTreeService(t *testing.T) {
	dt := []byte(testNoBuildOutOfTreeServiceYml)
	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Groups))
	require.Equal(t, 1, len(c.Targets))
}

func TestParseComposeTarget(t *testing.T) {
	dt := []byte(testParseComposeTargetYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)

	require.Equal(t, 2, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	require.Equal(t, "db", c.Targets[0].Name)
	require.Equal(t, "db", *c.Targets[0].Target)
	require.Equal(t, "webapp", c.Targets[1].Name)
	require.Equal(t, "webapp", *c.Targets[1].Target)
}

func TestComposeBuildWithoutContext(t *testing.T) {
	dt := []byte(testComposeBuildWithoutContextYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	require.Equal(t, "db", c.Targets[0].Name)
	require.Equal(t, "db", *c.Targets[0].Target)
	require.Equal(t, "webapp", c.Targets[1].Name)
	require.Equal(t, "webapp", *c.Targets[1].Target)
}

func TestBuildArgEnvCompose(t *testing.T) {
	dt := []byte(testBuildArgEnvComposeYml)

	t.Setenv("FOO", "bar")
	t.Setenv("BAR", "foo")
	t.Setenv("ZZZ_BAR", "zzz_foo")

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, sliceToMap(os.Environ()))
	require.NoError(t, err)
	require.Equal(t, ptrstr("bar"), c.Targets[0].Args["FOO"])
	require.Equal(t, ptrstr("zzz_foo"), c.Targets[0].Args["BAR"])
	require.Equal(t, ptrstr("FOO"), c.Targets[0].Args["BRB"])
}

func TestInconsistentComposeFile(t *testing.T) {
	dt := []byte(testInconsistentComposeFileYml)

	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `has neither an image nor a build context specified`)
}

func TestAdvancedNetwork(t *testing.T) {
	dt := []byte(testAdvancedNetworkYml)

	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestTags(t *testing.T) {
	dt := []byte(testTagsYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar"}, c.Targets[0].Tags)
}

func TestDependsOnList(t *testing.T) {
	dt := []byte(testDependsOnListYml)

	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestComposeExt(t *testing.T) {
	dt := []byte(testComposeExtYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	require.Equal(t, map[string]*string{"CT_ECR": ptrstr("foo"), "CT_TAG": ptrstr("bar")}, c.Targets[0].Args)
	require.Equal(t, []string{"ct-addon:baz", "ct-addon:foo", "ct-addon:alp"}, c.Targets[0].Tags)
	require.Equal(t, []string{"linux/amd64", "linux/arm64"}, c.Targets[0].Platforms)
	require.Equal(t, []string{"type=local,src=path/to/cache", "user/app:cache"}, stringify(c.Targets[0].CacheFrom))
	require.Equal(t, []string{"type=local,dest=path/to/cache", "user/app:cache"}, stringify(c.Targets[0].CacheTo))
	require.Equal(t, []string{"default", "key=/path/to/key", "other=path/to/otherkey"}, stringify(c.Targets[0].SSH))
	require.Equal(t, newBool(true), c.Targets[0].Pull)
	require.Equal(t, map[string]string{"alpine": "docker-image://alpine:3.13"}, c.Targets[0].Contexts)
	require.Equal(t, []string{"ct-fake-aws:bar"}, c.Targets[1].Tags)
	require.Equal(t, []string{"id=mysecret,src=/local/secret", "id=mysecret2,src=/local/secret2"}, stringify(c.Targets[1].Secrets))
	require.Equal(t, []string{"default"}, stringify(c.Targets[1].SSH))
	require.Equal(t, []string{"linux/arm64"}, c.Targets[1].Platforms)
	require.Equal(t, []string{"type=docker"}, stringify(c.Targets[1].Outputs))
	require.Equal(t, newBool(true), c.Targets[1].NoCache)
	require.Equal(t, ptrstr("128MiB"), c.Targets[1].ShmSize)
	require.Equal(t, []string{"nofile=1024:1024"}, c.Targets[1].Ulimits)
}

func TestComposeExtDedup(t *testing.T) {
	dt := []byte(testComposeExtDedupYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Targets))
	require.Equal(t, []string{"ct-addon:foo", "ct-addon:baz"}, c.Targets[0].Tags)
	require.Equal(t, []string{"type=local,src=path/to/cache", "user/app:cache"}, stringify(c.Targets[0].CacheFrom))
	require.Equal(t, []string{"type=local,dest=path/to/cache", "user/app:cache"}, stringify(c.Targets[0].CacheTo))
	require.Equal(t, []string{"default", "key=path/to/key"}, stringify(c.Targets[0].SSH))
}

func TestEnv(t *testing.T) {
	envf, err := os.CreateTemp("", "env")
	require.NoError(t, err)
	defer os.Remove(envf.Name())

	_, err = envf.WriteString("FOO=bsdf -csdf\n")
	require.NoError(t, err)

	dt := []byte(testEnvYml + envf.Name() + `
`)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]*string{"CT_ECR": ptrstr("foo"), "FOO": ptrstr("bsdf -csdf"), "NODE_ENV": ptrstr("test")}, c.Targets[0].Args)
}

func TestDotEnv(t *testing.T) {
	tmpdir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpdir, ".env"), []byte("FOO=bar"), 0644)
	require.NoError(t, err)

	dt := []byte(testDotEnvYml)

	chdir(t, tmpdir)
	c, err := ParseComposeFiles([]File{{
		Name: "docker-compose.yml",
		Data: dt,
	}}, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]*string{"FOO": ptrstr("bar")}, c.Targets[0].Args)
}

func TestPorts(t *testing.T) {
	dt := []byte(testPortsYml)
	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestPlatforms(t *testing.T) {
	dt := []byte(testPlatformsYml)
	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"linux/amd64", "linux/arm64"}, c.Targets[0].Platforms)
}

func newBool(val bool) *bool {
	b := val
	return &b
}

func TestServiceName(t *testing.T) {
	cases := []struct {
		svc     string
		wantErr bool
	}{
		{
			svc:     "a",
			wantErr: false,
		},
		{
			svc:     "abc",
			wantErr: false,
		},
		{
			svc:     "a.b",
			wantErr: false,
		},
		{
			svc:     "_a",
			wantErr: false,
		},
		{
			svc:     "a_b",
			wantErr: false,
		},
		{
			svc:     "AbC",
			wantErr: false,
		},
		{
			svc:     "AbC-0123",
			wantErr: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.svc, func(t *testing.T) {
			_, err := ParseCompose([]composetypes.ConfigFile{{Content: []byte(`
services:
  ` + tt.svc + `:
    build:
      context: .
`)}}, nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateComposeSecret(t *testing.T) {
	cases := []struct {
		name    string
		dt      []byte
		wantErr bool
	}{
		{
			name:    "secret set by file",
			dt:      []byte(testValidateComposeSecretYml),
			wantErr: false,
		},
		{
			name:    "secret set by environment",
			dt:      []byte(testValidateComposeSecretYml2),
			wantErr: false,
		},
		{
			name:    "external secret",
			dt:      []byte(testValidateComposeSecretYml3),
			wantErr: false,
		},
		{
			name: "unset secret",
			dt: []byte(`
secrets:
  foo: {}
`),
			wantErr: true,
		},
		{
			name:    "undefined secret",
			dt:      []byte(testValidateComposeSecretYml4),
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCompose([]composetypes.ConfigFile{{Content: tt.dt}}, nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateComposeFile(t *testing.T) {
	cases := []struct {
		name      string
		fn        string
		dt        []byte
		isCompose bool
		wantErr   bool
	}{
		{
			name: "empty service",
			fn:   "docker-compose.yml",
			dt: []byte(`
services:
  foo:
`),
			isCompose: true,
			wantErr:   false,
		},
		{
			name:      "build",
			fn:        "docker-compose.yml",
			dt:        []byte(testValidateComposeFileYml),
			isCompose: true,
			wantErr:   false,
		},
		{
			name:      "image",
			fn:        "docker-compose.yml",
			dt:        []byte(testValidateComposeFileYml2),
			isCompose: true,
			wantErr:   false,
		},
		{
			name:      "unknown ext",
			fn:        "docker-compose.foo",
			dt:        []byte(testValidateComposeFileYml3),
			isCompose: true,
			wantErr:   false,
		},
		{
			name: "hcl",
			fn:   "docker-bake.hcl",
			dt: []byte(`
target "default" {
  dockerfile = "test"
}
`),
			isCompose: false,
			wantErr:   false,
		},
		{
			name:      "json",
			fn:        "docker-bake.json",
			dt:        []byte(testValidateComposeFileJson),
			isCompose: false,
			wantErr:   false,
		},
		{
			name:      "json unknown ext",
			fn:        "docker-bake.foo",
			dt:        []byte(testValidateComposeFileJson2),
			isCompose: false,
			wantErr:   true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			isCompose, err := validateComposeFile(tt.dt, tt.fn, nil)
			assert.Equal(t, tt.isCompose, isCompose)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComposeNullArgs(t *testing.T) {
	dt := []byte(testComposeNullArgsYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]*string{"bar": ptrstr("baz")}, c.Targets[0].Args)
}

func TestDependsOn(t *testing.T) {
	dt := []byte(testDependsOnYml)
	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestInclude(t *testing.T) {
	tmpdir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpdir, "compose-foo.yml"), []byte(testIncludeYml), 0644)
	require.NoError(t, err)

	dt := []byte(testIncludeYml2)

	chdir(t, tmpdir)
	c, err := ParseComposeFiles([]File{{
		Name: "composetypes.yml",
		Data: dt,
	}}, nil)
	require.NoError(t, err)

	require.Equal(t, 2, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	require.Equal(t, "bar", c.Targets[0].Name)
	require.Equal(t, "buildbar", *c.Targets[0].Target)
	require.Equal(t, "foo", c.Targets[1].Name)
	require.Equal(t, "buildfoo", *c.Targets[1].Target)
}

func TestDevelop(t *testing.T) {
	dt := []byte(testDevelopYml)

	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestCgroup(t *testing.T) {
	dt := []byte(testCgroupYml)

	_, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
}

func TestProjectName(t *testing.T) {
	dt := []byte(testProjectNameYml)

	t.Run("default", func(t *testing.T) {
		c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
		require.NoError(t, err)
		require.Len(t, c.Targets, 1)
		require.Len(t, c.Targets[0].Args, 1)
		require.Equal(t, map[string]*string{"PROJECT_NAME": ptrstr("bake")}, c.Targets[0].Args)
	})

	t.Run("env", func(t *testing.T) {
		c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, map[string]string{"COMPOSE_PROJECT_NAME": "foo"})
		require.NoError(t, err)
		require.Len(t, c.Targets, 1)
		require.Len(t, c.Targets[0].Args, 1)
		require.Equal(t, map[string]*string{"PROJECT_NAME": ptrstr("foo")}, c.Targets[0].Args)
	})
}

func TestServiceContext(t *testing.T) {
	dt := []byte(testServiceContextYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.Groups))
	require.Equal(t, "default", c.Groups[0].Name)
	sort.Strings(c.Groups[0].Targets)
	require.Equal(t, []string{"base", "webapp"}, c.Groups[0].Targets)

	require.Equal(t, 2, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})

	require.Equal(t, "webapp", c.Targets[1].Name)
	require.Equal(t, map[string]string{"base": "target:base"}, c.Targets[1].Contexts)
}

func TestServiceContextDot(t *testing.T) {
	dt := []byte(testServiceContextDotYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.Groups))
	require.Equal(t, "default", c.Groups[0].Name)
	sort.Strings(c.Groups[0].Targets)
	require.Equal(t, []string{"base_1", "foo_1", "webapp"}, c.Groups[0].Targets)

	require.Equal(t, 3, len(c.Targets))
	sort.Slice(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})

	require.Equal(t, "webapp", c.Targets[2].Name)
	require.Equal(t, map[string]string{"base": "target:base_1", "foo": "target:foo_1"}, c.Targets[2].Contexts)
}

func TestDotEnvDir(t *testing.T) {
	tmpdir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(tmpdir, ".env"), 0755))

	dt := []byte(testDotEnvDirYml)

	chdir(t, tmpdir)
	_, err := ParseComposeFiles([]File{{Name: "compose.yml", Data: dt}}, nil)
	require.NoError(t, err)
}

func TestDotEnvEvaluate(t *testing.T) {
	tmpdir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpdir, ".env"), []byte(`
TEST_VALUE=${SYSTEM_VALUE:?system_value_not_set}
FOO_VALUE=${TEST_VALUE:?test_value_not_set}
`), 0644)
	require.NoError(t, err)

	dt := []byte(testDotEnvEvaluateYml)

	t.Setenv("SYSTEM_VALUE", "abc")

	chdir(t, tmpdir)
	c, err := ParseComposeFiles([]File{{Name: "compose.yml", Data: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]*string{"TEST_VALUE": ptrstr("abc"), "FOO_VALUE": ptrstr("abc")}, c.Targets[0].Args)
}

func TestUnknownField(t *testing.T) {
	tmpdir := t.TempDir()
	dt := []byte(testUnknownFieldYml)

	chdir(t, tmpdir)
	_, err := ParseComposeFiles([]File{{Name: "compose.yml", Data: dt}}, nil)
	require.NoError(t, err)
}

func TestUnknownBuildField(t *testing.T) {
	tmpdir := t.TempDir()
	dt := []byte(testUnknownBuildFieldYml)

	chdir(t, tmpdir)
	_, err := ParseComposeFiles([]File{{Name: "compose.yml", Data: dt}}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `additional properties 'foo' not allowed`)
}

func TestEmptyComposeFile(t *testing.T) {
	tmpdir := t.TempDir()
	chdir(t, tmpdir)
	_, err := ParseComposeFiles([]File{{Name: "compose.yml", Data: []byte(``)}}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `empty compose file`) // https://github.com/compose-spec/compose-go/blob/a42e7579d813e64c0c1f598a666358bc0c0a0eb4/loader/loader.go#L542
}

func TestParseComposeAttests(t *testing.T) {
	dt := []byte(testParseComposeAttestsYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Targets))

	target := c.Targets[0]
	require.Equal(t, "app", target.Name)

	require.NotNil(t, target.Attest)
	require.Len(t, target.Attest, 2)

	attestMap := target.Attest.ToMap()
	require.Contains(t, attestMap, "sbom")
	require.Contains(t, attestMap, "provenance")

	// Check the actual content - sbom=true should result in disabled=false (not disabled)
	require.Equal(t, "type=sbom", *attestMap["sbom"])
	require.Equal(t, "type=provenance,mode=max", *attestMap["provenance"])
}

func TestParseComposeAttestsDisabled(t *testing.T) {
	dt := []byte(testParseComposeAttestsDisabledYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Targets))

	target := c.Targets[0]
	require.Equal(t, "app", target.Name)

	require.NotNil(t, target.Attest)
	require.Len(t, target.Attest, 2)

	attestMap := target.Attest.ToMap()
	require.Contains(t, attestMap, "sbom")
	require.Contains(t, attestMap, "provenance")

	// When disabled=true, the value should be nil
	require.Nil(t, attestMap["sbom"])
	require.Nil(t, attestMap["provenance"])
}

func TestParseComposePull(t *testing.T) {
	dt := []byte(testParseComposePullYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Targets))

	target := c.Targets[0]
	require.Equal(t, "app", target.Name)
	require.Equal(t, true, *target.Pull)
}

func TestParseComposeNoCache(t *testing.T) {
	dt := []byte(testParseComposeNoCacheYml)

	c, err := ParseCompose([]composetypes.ConfigFile{{Content: dt}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Targets))

	target := c.Targets[0]
	require.Equal(t, "app", target.Name)
	require.Equal(t, true, *target.NoCache)
}

// chdir changes the current working directory to the named directory,
// and then restore the original working directory at the end of the test.
func chdir(t *testing.T, dir string) {
	olddir, err := os.Getwd()
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(olddir); err != nil {
			t.Errorf("chdir to original working directory %s: %v", olddir, err)
			os.Exit(1)
		}
	})
}

//go:embed testdata/test-parse-compose.yml
var testParseComposeYml string

//go:embed testdata/test-parse-compose-empty-cache-lists.yml
var testParseComposeEmptyCacheListsYml string

//go:embed testdata/test-no-build-out-of-tree-service.yml
var testNoBuildOutOfTreeServiceYml string

//go:embed testdata/test-parse-compose-target.yml
var testParseComposeTargetYml string

//go:embed testdata/test-compose-build-without-context.yml
var testComposeBuildWithoutContextYml string

//go:embed testdata/test-build-arg-env-compose.yml
var testBuildArgEnvComposeYml string

//go:embed testdata/test-inconsistent-compose-file.yml
var testInconsistentComposeFileYml string

//go:embed testdata/test-advanced-network.yml
var testAdvancedNetworkYml string

//go:embed testdata/test-tags.yml
var testTagsYml string

//go:embed testdata/test-depends-on-list.yml
var testDependsOnListYml string

//go:embed testdata/test-compose-ext.yml
var testComposeExtYml string

//go:embed testdata/test-compose-ext-dedup.yml
var testComposeExtDedupYml string

//go:embed testdata/test-env.yml
var testEnvYml string

//go:embed testdata/test-dot-env.yml
var testDotEnvYml string

//go:embed testdata/test-ports.yml
var testPortsYml string

//go:embed testdata/test-platforms.yml
var testPlatformsYml string

//go:embed testdata/test-validate-compose-secret.yml
var testValidateComposeSecretYml string

//go:embed testdata/test-validate-compose-secret-2.yml
var testValidateComposeSecretYml2 string

//go:embed testdata/test-validate-compose-secret-3.yml
var testValidateComposeSecretYml3 string

//go:embed testdata/test-validate-compose-secret-4.yml
var testValidateComposeSecretYml4 string

//go:embed testdata/test-validate-compose-file.yml
var testValidateComposeFileYml string

//go:embed testdata/test-validate-compose-file-2.yml
var testValidateComposeFileYml2 string

//go:embed testdata/test-validate-compose-file-3.yml
var testValidateComposeFileYml3 string

//go:embed testdata/test-validate-compose-file.json
var testValidateComposeFileJson string

//go:embed testdata/test-validate-compose-file-2.json
var testValidateComposeFileJson2 string

//go:embed testdata/test-compose-null-args.yml
var testComposeNullArgsYml string

//go:embed testdata/test-depends-on.yml
var testDependsOnYml string

//go:embed testdata/test-include.yml
var testIncludeYml string

//go:embed testdata/test-include-2.yml
var testIncludeYml2 string

//go:embed testdata/test-develop.yml
var testDevelopYml string

//go:embed testdata/test-cgroup.yml
var testCgroupYml string

//go:embed testdata/test-project-name.yml
var testProjectNameYml string

//go:embed testdata/test-service-context.yml
var testServiceContextYml string

//go:embed testdata/test-service-context-dot.yml
var testServiceContextDotYml string

//go:embed testdata/test-dot-env-dir.yml
var testDotEnvDirYml string

//go:embed testdata/test-dot-env-evaluate.yml
var testDotEnvEvaluateYml string

//go:embed testdata/test-unknown-field.yml
var testUnknownFieldYml string

//go:embed testdata/test-unknown-build-field.yml
var testUnknownBuildFieldYml string

//go:embed testdata/test-parse-compose-attests.yml
var testParseComposeAttestsYml string

//go:embed testdata/test-parse-compose-attests-disabled.yml
var testParseComposeAttestsDisabledYml string

//go:embed testdata/test-parse-compose-pull.yml
var testParseComposePullYml string

//go:embed testdata/test-parse-compose-no-cache.yml
var testParseComposeNoCacheYml string
