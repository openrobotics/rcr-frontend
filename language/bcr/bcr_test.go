package bcr

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectModuleNamesMergesExtraModulesDirs(t *testing.T) {
	repoRoot := t.TempDir()
	const registryRoot = "data/bazel-central-registry"

	mkdir := func(rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(repoRoot, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkdir(filepath.Join(registryRoot, "modules", "rclcpp"))
	mkdir(filepath.Join(registryRoot, "modules", "shared"))
	mkdir(filepath.Join(registryRoot, "bcr_staging", "modules", "imgui"))
	mkdir(filepath.Join(registryRoot, "bcr_staging", "modules", "ogre"))
	mkdir(filepath.Join(registryRoot, "bcr_staging", "modules", "shared")) // duplicate name
	mkdir(filepath.Join(registryRoot, "bcr_staging", "modules", ".hidden"))
	// A plain file must not be mistaken for a module dir.
	if err := os.WriteFile(filepath.Join(repoRoot, registryRoot, "bcr_staging", "modules", "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ext := &bcrExtension{
		repoRoot:          repoRoot,
		registryRoot:      registryRoot,
		modulesRoot:       filepath.Join(registryRoot, "modules"),
		extraModulesDirs:  stringSliceFlag{"bcr_staging/modules"},
		extraModulesRoots: []string{filepath.Join(registryRoot, "bcr_staging", "modules")},
	}

	got := ext.collectModuleNames([]string{"shared", "rclcpp"})
	want := []string{"imgui", "ogre", "rclcpp", "shared"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectModuleNames() = %v, want %v", got, want)
	}
}

func TestCollectModuleNamesMissingExtraDirIsIgnored(t *testing.T) {
	ext := &bcrExtension{
		repoRoot:          t.TempDir(),
		extraModulesRoots: []string{"registry/bcr_staging/modules"},
	}
	got := ext.collectModuleNames([]string{"b", "a"})
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectModuleNames() = %v, want %v", got, want)
	}
}

func TestModuleVersionDirFallsBackToModulesRoot(t *testing.T) {
	ext := &bcrExtension{
		modulesRoot:       "reg/modules",
		moduleVersionDirs: map[moduleID]string{newModuleID("imgui", "1.0"): "reg/bcr_staging/modules/imgui/1.0"},
	}
	if got, want := ext.moduleVersionDir("imgui", "1.0"), "reg/bcr_staging/modules/imgui/1.0"; got != want {
		t.Errorf("tracked dir = %q, want %q", got, want)
	}
	if got, want := ext.moduleVersionDir("rclcpp", "2.0"), "reg/modules/rclcpp/2.0"; got != want {
		t.Errorf("fallback dir = %q, want %q", got, want)
	}
}

func TestSkipDocsForExtraModuleVersions(t *testing.T) {
	primary := newModuleID("rclcpp", "1.0")
	staging := newModuleID("qtbase", "6.8.0")
	unknown := newModuleID("bazel_tools", "8.0.0") // pseudo-module, never visited
	ext := &bcrExtension{
		modulesRoot:       "reg/modules",
		extraModulesRoots: []string{"reg/bcr_staging/modules"},
		moduleVersionDirs: map[moduleID]string{
			primary: "reg/modules/rclcpp/1.0",
			staging: "reg/bcr_staging/modules/qtbase/6.8.0",
		},
	}

	if ext.isExtraModuleVersion(primary) {
		t.Error("primary module reported as extra")
	}
	if !ext.isExtraModuleVersion(staging) {
		t.Error("staging module not reported as extra")
	}
	if ext.isExtraModuleVersion(unknown) {
		t.Error("untracked module reported as extra")
	}

	// Default: extra-dir modules skip docs; everything else does not.
	if ext.skipDocsForModuleVersion(primary) || ext.skipDocsForModuleVersion(unknown) {
		t.Error("non-extra module should not skip docs")
	}
	if !ext.skipDocsForModuleVersion(staging) {
		t.Error("extra module should skip docs by default")
	}

	// Opt-in restores docs for extra-dir modules.
	ext.extraModulesDocs = true
	if ext.skipDocsForModuleVersion(staging) {
		t.Error("extra module should not skip docs when --extra-modules-docs is set")
	}
}

func TestStringSliceFlagSet(t *testing.T) {
	var f stringSliceFlag
	if err := f.Set("bcr_staging/modules/"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("other/modules"); err != nil {
		t.Fatal(err)
	}
	if got, want := []string(f), []string{"bcr_staging/modules", "other/modules"}; !reflect.DeepEqual(got, want) {
		t.Errorf("flag = %v, want %v", got, want)
	}
	if err := f.Set(""); err == nil {
		t.Error("expected error for empty value")
	}
}
