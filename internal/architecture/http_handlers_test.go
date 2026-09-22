package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestHTTPHandlersDoNotDispatchByRequestMethod(t *testing.T) {
	httpRoot := filepath.Join("..", "httpapi")
	err := filepath.WalkDir(httpRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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
		ast.Inspect(file, func(node ast.Node) bool {
			statement, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			selector, ok := statement.Tag.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "Method" {
				t.Errorf("%s dispatches HTTP methods inside a handler; register a dedicated chi handler instead", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPAPIContainsNoCyrillicText(t *testing.T) {
	httpRoot := filepath.Join("..", "httpapi")
	err := filepath.WalkDir(httpRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.IndexFunc(string(source), func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) >= 0 {
			t.Errorf("%s contains Cyrillic text; API messages must be in English", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
