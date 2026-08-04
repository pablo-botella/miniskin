// Command miniskin is the CLI for the miniskin build-time template assembler.
//
// Miniskin resolves percent-style template tags, applies skin overlays, and
// produces a set of files ready to be embedded into a Go binary via go:embed.
// See https://pkg.go.dev/github.com/pablo-botella/miniskin for the library API.
//
// # Usage
//
//	miniskin <command> [flags]
//
// # Commands
//
//   - run                    Mockup update + build embed assets + generate Go code.
//   - generate               Build embed assets + generate Go code (no mockup pass).
//   - debug                  Parse every catalog via cargoxml: -echo <f|-> writes the
//     byte-faithful rewrite, -rx <f|-> the claimed-vs-cargo report.
//   - mockup update          Export mockup pieces and refresh mockup-import blocks.
//   - mockup clean           Empty the inline content of mockup-import blocks.
//   - mockup negative        Transform a mockup file into a negative template.
//   - deps                   Print the dependency map and processing order.
//   - combine <dir>          Combine subdirectory .miniskin.xml files into one.
//   - split <file>           Split nested resource-lists into separate XML files.
//
// The mkskill generate family (-generate-claude-skill, -generate-agent-docs,
// -generate-readme) rides along via the embedded spec: the binary writes and
// installs its own composed docs, no mkskill around.
//
// # Common flags
//
//	-content string   path to the content directory (default ".")
//	-modules string   path to the modules directory (default ".")
//	-v                verbose output (dependency analysis, processing order)
//	-vv               debug output (all internal details)
//	-silent           suppress all output
//
// # Command-specific flags
//
// mockup negative:
//
//	-src string   source mockup file (required)
//	-dst string   destination negative template file (required)
//
// # Examples
//
// Run the full pipeline against the current directory:
//
//	miniskin run
//
// Inspect dependencies and processing order with verbose output:
//
//	miniskin deps -v
//
// Convert a mockup file into a negative template:
//
//	miniskin mockup negative -src page.html -dst page.tmpl.html
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablo-botella/miniskin"
	"github.com/pablo-botella/mskblob"
)

func main() {

	// mkskill  checks
	if err, done := MkskillSpec.CheckParams(); done {
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		usage()
	}

	cmd := os.Args[1]
	argsOffset := 2

	// Handle "mockup" subcommands
	if cmd == "mockup" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin mockup <update|clean|negative> [flags]\n")
			os.Exit(1)
		}
		cmd = "mockup " + os.Args[2]
		argsOffset = 3
	}

	// Validate command
	switch cmd {
	case "run", "generate", "debug", "mockup update", "mockup clean", "mockup negative", "deps", "combine", "split", "blob-header":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
	}

	// Parse flags after the command
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	contentPath := fs.String("content", ".", "path to content directory")
	modulesPath := fs.String("modules", ".", "path to modules directory")
	verbose := fs.Bool("v", false, "verbose output (dependency analysis, processing order)")
	debug := fs.Bool("vv", false, "debug output (all internal details)")
	silent := fs.Bool("silent", false, "suppress all output")

	// Flags exclusive to mockup negative
	var negativeSrc, negativeDst *string
	if cmd == "mockup negative" {
		negativeSrc = fs.String("src", "", "source mockup file (required)")
		negativeDst = fs.String("dst", "", "destination negative template file (required)")
	}

	// Flags exclusive to debug: one parameter per output
	var debugEcho, debugRx *string
	if cmd == "debug" {
		debugEcho = fs.String("echo", "", "byte-faithful rewrite of every catalog (file, or - for stdout)")
		debugRx = fs.String("rx", "", "claimed-vs-cargo report (file, or - for stdout; default when no output is chosen)")
	}

	fs.Parse(os.Args[argsOffset:])

	verbosity := miniskin.VerbosityNormal
	if *debug {
		verbosity = miniskin.VerbosityDebug
	} else if *verbose {
		verbosity = miniskin.VerbosityVerbose
	} else if *silent {
		verbosity = miniskin.VerbositySilent
	}

	var err error
	switch cmd {
	case "run":
		err = miniskin.MiniskinRun(*contentPath, *modulesPath, verbosity)
	case "generate":
		err = miniskin.MiniskinGenerate(*contentPath, *modulesPath, verbosity)
	case "debug":
		err = miniskin.DebugCatalogs(*contentPath, *debugEcho, *debugRx)
	case "mockup update":
		err = miniskin.MiniskinMockupUpdate(*contentPath, *modulesPath, verbosity)
	case "mockup clean":
		err = miniskin.MiniskinMockupClean(*contentPath, *modulesPath, verbosity)
	case "mockup negative":
		if *negativeSrc == "" || *negativeDst == "" {
			fmt.Fprintf(os.Stderr, "mockup negative requires -src and -dst flags\n")
			os.Exit(1)
		}
		absSrc, _ := filepath.Abs(*negativeSrc)
		absDst, _ := filepath.Abs(*negativeDst)
		var data []byte
		data, err = os.ReadFile(absSrc)
		if err != nil {
			break
		}
		result := miniskin.TransformNegative(string(data))
		err = os.WriteFile(absDst, []byte(result), 0644)
		if err == nil {
			fmt.Printf("negative: %s -> %s\n", *negativeSrc, *negativeDst)
		}
	case "deps":
		ms := miniskin.MiniskinNew(*contentPath, *modulesPath).SetVerbosity(verbosity)
		var dm *miniskin.DepMap
		dm, err = ms.AnalyzeDeps()
		if err == nil {
			fmt.Print(dm.String())
			order, orderErr := dm.ProcessingOrder()
			if orderErr != nil {
				err = orderErr
			} else {
				fmt.Println("\n=== Processing Order ===")
				for i, src := range order {
					fmt.Printf("  %d. %s\n", i+1, src)
				}
			}
		}
	case "combine":
		args := fs.Args()
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin combine <directory>\n")
			os.Exit(1)
		}
		err = miniskin.CombineDir(args[0])
		if err == nil {
			fmt.Printf("combined: %s\n", args[0])
		}
	case "split":
		args := fs.Args()
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin split <file.miniskin.xml>\n")
			os.Exit(1)
		}
		err = miniskin.SplitXML(args[0])
		if err == nil {
			fmt.Printf("split: %s\n", args[0])
		}
	case "blob-header":
		args := fs.Args()
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin blob-header <file.blob>\n")
			os.Exit(1)
		}
		var hdr mskblob.Header
		hdr, err = mskblob.ReadHeader(args[0])
		if err == nil {
			fmt.Printf("blob:    %s\n", args[0])
			fmt.Printf("magic:   MSPK\n")
			fmt.Printf("version: %d\n", hdr.Version)
			fmt.Printf("id:      %s\n", hdr.ID)
			fmt.Printf("entries: %d\n", hdr.Count)
			fmt.Printf("dataCRC: %#08x\n", hdr.DataCRC32)
			fmt.Printf("data:    %d bytes\n", hdr.DataSize)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {

	fmt.Fprintf(os.Stderr, "Usage: miniskin <command> [flags]\n\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  run                    Mockup update + Build + Generate code\n")
	fmt.Fprintf(os.Stderr, "  generate               Build embed assets + Generate Go code\n")
	fmt.Fprintf(os.Stderr, "  %s\n", strings.ReplaceAll(MkskillSpec.Usage(true), "\n", "\n  "))
	fmt.Fprintf(os.Stderr, "  debug                  Cargo parse of every catalog: -echo <f|-> faithful rewrite, -rx <f|-> report\n")
	fmt.Fprintf(os.Stderr, "  mockup update          Export mockup pieces + Refresh imports\n")
	fmt.Fprintf(os.Stderr, "  mockup clean           Empty inline content of mockup-import blocks\n")
	fmt.Fprintf(os.Stderr, "  mockup negative        Transform a mockup file into a negative template\n")
	fmt.Fprintf(os.Stderr, "  deps                   Show dependency map and processing order\n")
	fmt.Fprintf(os.Stderr, "  combine <dir>          Combine subdirectory XMLs into one\n")
	fmt.Fprintf(os.Stderr, "  split <file>           Split nested resource-lists into separate XMLs\n")
	fmt.Fprintf(os.Stderr, "  blob-header <file>     Inspect a .blob file's header (magic, version, buildID)\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -content string        path to content directory (default \".\")\n")
	fmt.Fprintf(os.Stderr, "  -modules string        path to modules directory (default \".\")\n")
	fmt.Fprintf(os.Stderr, "  -v                     verbose output\n")
	fmt.Fprintf(os.Stderr, "  -vv                    debug output\n")
	fmt.Fprintf(os.Stderr, "  -silent                suppress all output\n")
	fmt.Fprintf(os.Stderr, "\nMockup negative flags:\n")
	fmt.Fprintf(os.Stderr, "  -src string            source mockup file (required)\n")
	fmt.Fprintf(os.Stderr, "  -dst string            destination negative template file (required)\n")
	os.Exit(1)
}
