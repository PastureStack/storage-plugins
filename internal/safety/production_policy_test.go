package safety

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	moduleImportPath    = "github.com/PastureStack/storage-plugins"
	cliEntryPath        = "cmd/storage-plugins/main.go"
	gitMetadataRootPath = ".git"
)

var allowedProductionImports = map[string]struct{}{
	"bytes":         {},
	"crypto/sha256": {},
	"embed":         {},
	"encoding/hex":  {},
	"encoding/json": {},
	"errors":        {},
	"flag":          {},
	"fmt":           {},
	"io":            {},
	"os":            {},
	"regexp":        {},
	"sort":          {},
	"unicode":       {},
	"unicode/utf8":  {},
}

var allowedOSSelectors = map[string]struct{}{
	"Args":   {},
	"Exit":   {},
	"Stderr": {},
	"Stdin":  {},
	"Stdout": {},
}

func TestProductionGoPolicyCoversModuleRoot(t *testing.T) {
	findings, err := scanProductionGo(moduleRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("production policy findings:\n%s", strings.Join(findings, "\n"))
	}
}

func TestProductionGoPolicyRejectsAliasedOSSelectors(t *testing.T) {
	for _, selector := range []string{"Chmod", "Create", "File", "NewFile", "Open", "OpenFile", "Remove", "Rename", "StartProcess", "WriteFile"} {
		t.Run(selector, func(t *testing.T) {
			root := t.TempDir()
			writeGoFixture(t, root, cliEntryPath, "package main\nimport system \"os\"\nvar _ = system."+selector+"\n")
			findings, err := scanProductionGo(root)
			if err != nil {
				t.Fatal(err)
			}
			assertFinding(t, findings, cliEntryPath, "os selector is not allowlisted: "+selector)
		})
	}
}

func TestProductionGoPolicyRejectsNestedNewPackageImports(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "pkg/evil.go", "package pkg\nimport (\n\t\"net\"\n\t\"os/exec\"\n)\nvar _, _ = net.Dial, exec.Command\n")
	writeGoFixture(t, root, "newpkg/deep/evil.go", "package deep\nimport \"net\"\nvar _ = net.Dial\n")
	writeGoFixture(t, root, "pkg/cache/evil.go", "package cache\nimport \"net\"\nvar _ = net.Dial\n")

	findings, err := scanProductionGo(root)
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, findings, "pkg/evil.go", `production import is not allowlisted: "net"`)
	assertFinding(t, findings, "pkg/evil.go", `production import is not allowlisted: "os/exec"`)
	assertFinding(t, findings, "newpkg/deep/evil.go", `production import is not allowlisted: "net"`)
	assertFinding(t, findings, "pkg/cache/evil.go", `production import is not allowlisted: "net"`)
}

func TestProductionGoPolicyRejectsOSOutsideCLI(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "pkg/process.go", "package pkg\nimport system \"os\"\nvar _ = system.Args\n")
	findings, err := scanProductionGo(root)
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, findings, "pkg/process.go", "os import is allowed only in "+cliEntryPath)
}

func TestProductionGoPolicyRejectsDotAndBlankOSImports(t *testing.T) {
	for _, alias := range []string{".", "_"} {
		t.Run(alias, func(t *testing.T) {
			root := t.TempDir()
			writeGoFixture(t, root, cliEntryPath, "package main\nimport "+alias+" \"os\"\nfunc main() {}\n")
			findings, err := scanProductionGo(root)
			if err != nil {
				t.Fatal(err)
			}
			assertFinding(t, findings, cliEntryPath, "dot or blank os import is not allowed")
		})
	}
}

func TestProductionGoPolicyAllowsRequiredCLISelectors(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, cliEntryPath, "package main\nimport \"os\"\nfunc main() {\n\t_ = os.Args\n\t_ = os.Stdin\n\t_ = os.Stdout\n\t_ = os.Stderr\n\tos.Exit(0)\n}\n")
	findings, err := scanProductionGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("required CLI selectors were rejected:\n%s", strings.Join(findings, "\n"))
	}
}

func TestProductionGoPolicySkipsOnlyGitMetadataAndTests(t *testing.T) {
	root := t.TempDir()
	evil := "package ignored\nimport \"os/exec\"\nvar _ = exec.Command\n"
	for _, relative := range []string{".git/evil.go", "pkg/evil_test.go"} {
		writeGoFixture(t, root, relative, evil)
	}
	findings, err := scanProductionGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("non-production files were scanned:\n%s", strings.Join(findings, "\n"))
	}
}

func TestProductionGoPolicyScansBinCacheAndGeneratedSource(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "bin/evil.go", "package bin\nimport \"os/exec\"\nvar _ = exec.Command\n")
	writeGoFixture(t, root, "cache/deep/evil.go", "package deep\nimport \"net\"\nvar _ = net.Dial\n")
	writeGoFixture(t, root, "generated/unsafe_generated.go", "// Code generated for a policy fixture. DO NOT EDIT.\npackage generated\nimport \"net\"\nvar _ = net.Dial\n")

	findings, err := scanProductionGo(root)
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, findings, "bin/evil.go", `production import is not allowlisted: "os/exec"`)
	assertFinding(t, findings, "cache/deep/evil.go", `production import is not allowlisted: "net"`)
	assertFinding(t, findings, "generated/unsafe_generated.go", `production import is not allowlisted: "net"`)
}

func scanProductionGo(root string) ([]string, error) {
	root = filepath.Clean(root)
	findings := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				relativeDirectory, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				if filepath.ToSlash(relativeDirectory) == gitMetadataRootPath {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
		if err != nil {
			return fmt.Errorf("parse production Go file %s: %w", relative, err)
		}
		findings = append(findings, inspectProductionFile(relative, parsed)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(findings)
	return findings, nil
}

func inspectProductionFile(relative string, parsed *ast.File) []string {
	findings := []string{}
	osAliases := map[string]struct{}{}
	for _, imported := range parsed.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			findings = append(findings, relative+": invalid import path")
			continue
		}
		if !isAllowedProductionImport(importPath) {
			findings = append(findings, fmt.Sprintf("%s: production import is not allowlisted: %q", relative, importPath))
		}
		if importPath != "os" {
			continue
		}
		if relative != cliEntryPath {
			findings = append(findings, relative+": os import is allowed only in "+cliEntryPath)
		}
		alias := "os"
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias == "." || alias == "_" {
			findings = append(findings, relative+": dot or blank os import is not allowed")
			continue
		}
		osAliases[alias] = struct{}{}
	}

	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, importedOS := osAliases[identifier.Name]; !importedOS {
			return true
		}
		if _, allowed := allowedOSSelectors[selector.Sel.Name]; !allowed {
			findings = append(findings, relative+": os selector is not allowlisted: "+selector.Sel.Name)
		}
		return true
	})
	return findings
}

func isAllowedProductionImport(importPath string) bool {
	if strings.HasPrefix(importPath, moduleImportPath+"/") {
		return true
	}
	_, allowed := allowedProductionImports[importPath]
	return allowed
}

func writeGoFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFinding(t *testing.T, findings []string, path, message string) {
	t.Helper()
	want := filepath.ToSlash(path) + ": " + message
	for _, finding := range findings {
		if finding == want {
			return
		}
	}
	t.Fatalf("finding %q not found in:\n%s", want, strings.Join(findings, "\n"))
}
