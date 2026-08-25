package volumeplugin

import (
	"path/filepath"
	"testing"
)

func TestMountDestinationRejectsUnsafeVolumeNames(t *testing.T) {
	driver := &ControlPlaneStorageDriver{Basedir: filepath.Join("var", "lib", "pasturestack", "volumes"), DriverName: "test-driver"}
	destination, err := driver.getMntDest("data_volume-01")
	if err != nil {
		t.Fatal(err)
	}
	if !pathWithinRoot(driver.getMntRoot(), destination) {
		t.Fatalf("safe destination escaped root: %q", destination)
	}

	for _, name := range []string{"", ".", "..", "../escape", "a/b", `a\b`, "bad\x00name", "bad\nname"} {
		if _, err := driver.getMntDest(name); err == nil {
			t.Errorf("unsafe volume name %q was accepted", name)
		}
	}
}

func TestPathWithinRootRejectsRootParentAndSibling(t *testing.T) {
	root := filepath.Join("var", "lib", "pasturestack", "volumes", "driver")
	if !pathWithinRoot(root, filepath.Join(root, "volume")) {
		t.Fatal("child path was rejected")
	}
	for _, candidate := range []string{root, filepath.Dir(root), root + "-sibling"} {
		if pathWithinRoot(root, candidate) {
			t.Errorf("non-child path %q was accepted", candidate)
		}
	}
}

func TestKeyedLockerReleasesUnusedKeys(t *testing.T) {
	locker := newKeyedLocker()
	locker.Lock("volume")
	locker.Unlock("volume")
	if len(locker.locks) != 0 {
		t.Fatalf("unused keyed locks retained: %d", len(locker.locks))
	}
}
