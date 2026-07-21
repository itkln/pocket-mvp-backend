package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDomainModulesDoNotImportEachOther(t *testing.T) {
	modulesRoot := filepath.Join("..", "modules")
	err := filepath.WalkDir(modulesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		currentModule := moduleFromPath(path)
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			const prefix = "pocket-mvp-backend/internal/modules/"
			if !strings.HasPrefix(importPath, prefix) {
				continue
			}
			importedModule := strings.Split(strings.TrimPrefix(importPath, prefix), "/")[0]
			if importedModule != currentModule {
				t.Errorf("%s imports another domain module %q", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func moduleFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/modules/")
	if len(parts) != 2 {
		return ""
	}
	return strings.Split(parts[1], "/")[0]
}
