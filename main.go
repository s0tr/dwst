package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Options struct {
	Path      string
	CachePath string
	Printer   Printer
	Filter    Filter
}

var opts Options

func parseOptions() {
	var sizeRange string
	var offsetType string
	flag.StringVar(&opts.Path, "path", "", "binary path")
	flag.StringVar(&opts.CachePath, "cache-path", "", "saves all structures to gob.gz file")
	flag.BoolVar(&opts.Printer.Expand, "expand", false, "expand nested structs")
	flag.BoolVar(&opts.Printer.PrintAsJson, "json", false, "print as json")
	flag.BoolVar(&opts.Printer.Hex, "hex", false, "print numbers as hex")
	flag.StringVar(&opts.Filter.StructName, "name", "", "filter by structure name")
	flag.Int64Var(&opts.Filter.SizeLow, "size-low", -1, "filter by minimum structure size")
	flag.Lookup("size-low").DefValue = "0"
	flag.Int64Var(&opts.Filter.SizeHigh, "size-high", -1, "filter by maximum structure size")
	flag.Lookup("size-high").DefValue = "0"
	flag.StringVar(&sizeRange, "size-range", "", "filter structure by size range")
	flag.StringVar(&offsetType, "field-offset-type", "", "filter structure by offset and field name")
	flag.BoolVar(&opts.Filter.Exact, "exact", false, "strings must match exactly")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DWST is an utility for visualizing data structures in binaries containing dwarf info\n")
		fmt.Fprintf(os.Stderr, "Usage: dwst [options] <path>\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  dwst --cache-path cache.gob.gz ./binary\n")
		fmt.Fprintf(os.Stderr, "  dwst --name MyStruct ./binary\n")
		fmt.Fprintf(os.Stderr, "  dwst --size-range 62,126 ./binary\n")
		fmt.Fprintf(os.Stderr, "  dwst --size-low 62 --size-high 126 ./binary\n")
		fmt.Fprintf(os.Stderr, "  dwst --field-offset-type 24,\"struct ucred\" --expand ./binary\n")

	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		log.Fatal("!! no binary path was provided")
	}
	opts.Path = args[0]

	_, err := os.Stat(opts.Path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("!! file [%s] does not exist", opts.Path)
	}

	if sizeRange != "" {
		parts := strings.SplitN(sizeRange, ",", 2)
		if len(parts) != 2 {
			log.Fatal("!! invalid format for --size-range, expected: int,int")
		}
		sizeLow, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			log.Fatal("!! invalid format for --size-range, expected: int,int")
		}
		sizeHigh, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			log.Fatal("!! invalid format for --size-range, expected: int,int")
		}
		opts.Filter.SizeLow = sizeLow
		opts.Filter.SizeHigh = sizeHigh
	}

	if offsetType != "" {
		parts := strings.SplitN(offsetType, ",", 2)
		if len(parts) != 2 {
			log.Fatal("!! invalid  format for --field-offset-type, expected: int,string")
		}
		offset, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			log.Fatal("!! invalid  format for --field-offset-type, expected: int,string")
		}
		name := strings.TrimSpace(parts[1])
		name = strings.Trim(name, `"'`)
		opts.Filter.FieldOffset = offset
		opts.Filter.FieldName = name
	}
}

func printStruct(structInfo *StructInfo) error {
	if opts.Filter.Match(structInfo) {
		opts.Printer.Print(structInfo)
	}
	return nil
}

func main() {
	parseOptions()
	parser, err := NewParser(opts.Path)
	if err != nil {
		log.Fatal(err)
	}
	if opts.CachePath != "" {
		err = parser.CacheStructs(opts.CachePath)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = parser.IterateStructInfoWithCallback(printStruct)
		if err != nil {
			log.Fatal(err)
		}
	}
}
