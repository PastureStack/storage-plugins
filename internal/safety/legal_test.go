package safety

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

const (
	historicalName  = "Ran" + "cher"
	historicalOwner = "SU" + "SE"
)

var requiredDisclaimer = "PastureStack is an independent community effort to preserve, audit, and modernize the " + historicalName + " 1.6 ecosystem. It is not affiliated with or endorsed by " + historicalName + " Labs or " + historicalOwner + "."

func TestRequiredDisclaimerIsTheFirstParagraph(t *testing.T) {
	file, err := os.Open(filepath.Join(moduleRoot(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("README is empty")
	}
	if scanner.Text() != requiredDisclaimer {
		t.Fatalf("first paragraph changed: %q", scanner.Text())
	}
}

func TestRequiredLegalFilesHaveExactHashes(t *testing.T) {
	tests := []struct {
		path string
		size int
		hash string
	}{
		{path: "LICENSE", size: 10351, hash: "eb3d7b5485466acbd81f2b496f595ab637d2792e268206b27d99e793bdb67549"},
		{path: "LICENSES/GO-LICENSE", size: 1453, hash: "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"},
		{path: "LICENSES/GO-PATENTS", size: 1303, hash: "96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc"},
	}
	for _, test := range tests {
		data, err := os.ReadFile(filepath.Join(moduleRoot(t), filepath.FromSlash(test.path)))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != test.size {
			t.Fatalf("%s size=%d, want %d", test.path, len(data), test.size)
		}
		digest := sha256.Sum256(data)
		if actual := hex.EncodeToString(digest[:]); actual != test.hash {
			t.Fatalf("%s hash=%s, want %s", test.path, actual, test.hash)
		}
	}
}
