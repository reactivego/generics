package pkg

import (
	"bytes"
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

// ScanForGeneratedSources looks in the comments of the file for comment pragma //jig:name and
// adds the name to the list of generated source fragments. This allows multiple source fragments to
// be stored in a single file.
func (p *Package) ScanForGeneratedSources(file *ast.File) {
	for _, cgroup := range file.Comments {
		for _, comment := range cgroup.List {
			if strings.HasPrefix(comment.Text, jigName) {
				kvmatch := reCommentPragma.FindStringSubmatch(comment.Text)
				if len(kvmatch) == 3 {
					if kvmatch[1] == jigName {
						path := p.Filepath(file)
						p.generated[kvmatch[2]] = path
						p.fileset[path] = file
					}
				}
			}
		}
	}
}

// HasGeneratedSource is used in the templ.PackageWriter interface to prevent
// fragments from being generated multiple times.
func (p *Package) HasGeneratedSource(name string) bool {
	_, present := p.generated[name]
	return present
}

// GenerateSource will take the passed name and source and add it to the package.
// Currently implemented in such way that every source fragment will be written
// to its own file. Eventually you want multiple generated fragments to share
// a physical file on disk. Especially when you have more that e.g. 50 generated
// source fragments.
func (p *Package) GenerateSource(packageName, name, source string) error {
	return p.GenerateSourceAppendFile(p.filename, packageName, name, source)
}

// GenerateSourceAppendFile will generate the source and append it to a
// shared source file. Duh!
func (p *Package) GenerateSourceAppendFile(filename *template.Template, packageName, name, source string) error {
	sourcebuf := &bytes.Buffer{}
	data := map[string]string{
		"Package": strings.Title(packageName),
		"package": strings.ToLower(packageName),
		"Name":    strings.Title(name),
		"name":    strings.ToLower(name),
	}
	filenamebuf := &bytes.Buffer{}
	filename.Execute(filenamebuf, data)
	path := filepath.Join(p.Dir, filenamebuf.String())
	file, present := p.fileset[path]
	if present {
		err := p.WriteFile(sourcebuf, file)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(sourcebuf, "// Code generated by jig; DO NOT EDIT.\n\n//go:generate jig --regen\n\npackage %v\n\n", p.Name)
	}

	// Append the source fragment to the source.
	fmt.Fprintf(sourcebuf, "\n%s %s\n\n%v", jigName, name, source)

	// Rewrite imports clause for the source.
	fixedsource, err := imports.Process("", sourcebuf.Bytes(), nil)
	if err != nil {
		return err
	}

	file, err = p.Config.ParseFile(path, string(fixedsource))
	if err != nil {
		return err
	}

	// Add file to the fileset, idempotent
	p.AddFile(file)

	// Remember this file as the most current one.
	p.generated[name] = path
	return nil
}
