package build

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// RewriteImports parses a Go file, rewrites tier 2 imports per rewriteMap,
// and writes the result to destPath.
func RewriteImports(srcPath, destPath string, rewriteMap map[string]string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// Rewrite import paths
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if newPath, ok := rewriteMap[path]; ok {
			imp.Path.Value = `"` + newPath + `"`

			// Also update the EndPos in the AST's import map
			// This ensures go/printer outputs the new path
		}
	}

	// Also rewrite in the Decls for completeness (parser sometimes keeps separate references)
	ast.Inspect(f, func(n ast.Node) bool {
		if imp, ok := n.(*ast.ImportSpec); ok {
			path := strings.Trim(imp.Path.Value, `"`)
			if newPath, ok := rewriteMap[path]; ok {
				imp.Path.Value = `"` + newPath + `"`
			}
		}
		return true
	})

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return printer.Fprint(out, fset, f)
}
