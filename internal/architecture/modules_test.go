package architecture

import (
	"go/ast"
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

func TestPersistenceImportsStayInRepositoryAdapters(t *testing.T) {
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
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			name := filepath.Base(path)
			if (strings.HasPrefix(importPath, "github.com/jackc/pgx") || importPath == "pocket-mvp-backend/internal/orderstore") &&
				!(strings.HasPrefix(name, "repository_") && strings.HasSuffix(name, "_postgres.go")) {
				t.Errorf("%s imports persistence outside a repository adapter", path)
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

// The HTTP layer talks to modules only through their ports, wired in bootstrap: it must not reach
// the database or build repositories itself (a service package once did both and slipped past
// the module checks).
func TestHTTPLayerUsesModulePortsOnly(t *testing.T) {
	allowed := []string{"pocket-mvp-backend/internal/modules/", "pocket-mvp-backend/internal/appfault", "pocket-mvp-backend/internal/buildinfo"}
	root := filepath.Join("..", "httpapi")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(importPath, "github.com/jackc/pgx") {
				t.Errorf("%s imports the database driver %q", path, importPath)
			}
			if strings.HasPrefix(importPath, "pocket-mvp-backend/internal/") {
				ok := false
				for _, prefix := range allowed {
					ok = ok || strings.HasPrefix(importPath, prefix)
				}
				if !ok {
					t.Errorf("%s imports %q; HTTP handlers may use only module ports, appfault and buildinfo", path, importPath)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && strings.HasPrefix(selector.Sel.Name, "New") && strings.HasSuffix(selector.Sel.Name, "Repository") {
				t.Errorf("%s builds a repository (%s); wire it in bootstrap", path, selector.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
