// yajsv is a command line tool for validating JSON documents against
// a provided JSON Schema - https://json-schema.org/
package main

import (
	"flag"
    "fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/xeipuuv/gojsonschema"
)

var (
	schemaFlag = flag.String("s", "", "primary JSON schema to validate against")
	refFlags stringFlags
	quietFlag = flag.Bool("q", false, "quiet, skip printing valid JSON documents")
)

func init() {
	flag.Var(&refFlags, "r", "referenced schema(s), can be globs and/or used multiple times")
	flag.Usage = printUsage
}

func main() {
	flag.Parse()
	if *schemaFlag == "" {
		printUsage()
		os.Exit(2)
	}

	sl := gojsonschema.NewSchemaLoader()
	schemaUri := fileUri(*schemaFlag)

	for _, ref := range refFlags {
		for _, p := range glob(ref) {
			uri := fileUri(p)
			if uri == schemaUri {
				continue
			}
			loader := gojsonschema.NewReferenceLoader(uri)
			err := sl.AddSchemas(loader)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

    schemaLoader := gojsonschema.NewReferenceLoader(schemaUri)
	schema, err := sl.Compile(schemaLoader)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	// Limit the number of simultaneously open files to avoid ulimit issues
	sem := make(chan int, runtime.GOMAXPROCS(0)+10)

	for _, arg := range flag.Args() {
		for _, p := range glob(arg) {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				sem <- 0
				defer func() { <-sem }()

				loader := gojsonschema.NewReferenceLoader(fileUri(path))
				result, err := schema.Validate(loader)
				switch {
				case err != nil:
					fmt.Printf("%s: error: %s\n", path, err)
				case !result.Valid():
					lines := make([]string, len(result.Errors()))
					for i, desc := range result.Errors() {
						lines[i] = fmt.Sprintf("%s: invalid: %s", path, desc)
					}
					fmt.Println(strings.Join(lines, "\n"))
				case !*quietFlag:
					fmt.Printf("%s: ok\n", path)
				}
			}(p)
		}
	}
	wg.Wait()

	// TODO Summary of invalid/error and proper exit code
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s -s schema.json [-r ref-schema.json -r ...] document.json [...]\n\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
}

func fileUri(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return "file://" + abs
}

// glob is a wrapper that also resolves `~` since we may be skipping
// the shell expansion when single-quoting globs at the command line
func glob(pattern string) []string {
	pattern, err := homedir.Expand(pattern)
	if err != nil {
		log.Fatal(err)
	}
	paths, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatal(err)
	}
	return paths
}

type stringFlags []string

func (sf *stringFlags) String() string {
    return "multi-string"
}

func (sf *stringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}
