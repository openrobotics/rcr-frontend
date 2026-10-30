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
