/*
jig is a tool for generating Go source code from a generics library.

  $ go get github.com/reactivego/jig
  $ jig -h
  Usage of jig [flags] [<dir>]:
  -c, --clean     Remove files generated by jig
  -m, --missing   Only generate code that is missing
  -r, --regen     Force regeneration of all code by jig (default)
  -v, --verbose   Print details of what jig is doing

For details see https://github.com/reactivego/jig/
*/
package main

import (
	"fmt"
	"os"

	"github.com/reactivego/jig/pkg"
	"github.com/reactivego/jig/templ"

	"github.com/spf13/pflag"
)

func main() {
	os.Exit(jigMain())
}

func jigMain() int {
	// Flag handling...
	var clean, forceregen, missing, verbose, nodoc bool
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s [flags] [<dir>]:\n", os.Args[0])
		pflag.PrintDefaults()
	}
	pflag.BoolVarP(&clean, "clean", "c", false, "Remove files generated by jig")
    pflag.BoolVarP(&forceregen, "regen", "r", false, "Force regeneration of all code by jig (default)")
	pflag.BoolVarP(&missing, "missing", "m", false, "Only generate code that is missing")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Print details of what jig is doing")
	pflag.BoolVarP(&nodoc, "nodoc","n", false, "No documentation in generated files")
	pflag.Parse()

	if forceregen && missing {
		missing = false
	}

	// If no dir has been given, use current directory.
	dir := pflag.Arg(0)
	if dir == "" {
		dir = "."
	}

	// Create a package that will read and write files from the given dir.
	pkg := pkg.NewPackage(dir)
	pkg.Nodoc = nodoc
	
	// Parse all files currently in the package directory.
	err := pkg.ParseDir()
	if printedError(verbose, nil, err) {
		return 1
	}

	if clean || !missing {
		// Clean the output directory by removing all generated source code file(s)
		messages, err := pkg.RemoveGeneratedSources()
		if printedError(verbose, messages, err) {
			return 1
		}
		if clean {
			return 0
		}
	}

	write := true

	var (
		errors []error
		tplr   templ.Specializer
	)

	// As long as files are being generated we are still fixing code.
	for generating := write; generating; {
		generating = false

		errors, err = pkg.Check() // ~ 410ms
		if printedError(verbose, nil, err) {
			return 1
		}

		if len(errors) == 0 {
			break
		}

		// Look in the files directly associated with the package for
		// comment pragmas jig:file and jig:type.
		messages := pkg.LoadGeneratePragmas()
		if printedError(verbose, messages, nil) {
			return 1
		}

		if tplr == nil {
			// Look for our //jigs: comment pragmas and import
			// any templates declared that way.
			tplr = templ.NewSpecializer()
			messages, err := pkg.LoadGenerics(tplr) // ~2ms
			if printedError(verbose, messages, err) {
				return 1
			}
		}

		// Implement missing language constructs.
		for _, sig := range pkg.SuggestTypesToGenerate(errors) {
			messages, err := tplr.GenerateCodeForType(pkg, sig)
			if printedError(verbose, messages, err) {
				return 1
			}
			generating = generating || len(messages) > 0
		}
	}

	if write {
		// Write the generated source code file(s)
		messages, err := pkg.WriteGeneratedSources()
		if printedError(verbose, messages, err) {
			return 1
		}
	}

	// Print unfixable errors from the last time Check() was called.
	for _, err := range errors {
		fmt.Println(err)
	}
	if len(errors) > 0 {
		return 1
	}

	return 0
}

func printedError(verbose bool, messages []string, err error) bool {
	if verbose {
		for _, msg := range messages {
			fmt.Println(msg)
		}
	}
	if err == nil {
		return false
	}
	fmt.Fprintln(os.Stderr, err)
	return true
}
