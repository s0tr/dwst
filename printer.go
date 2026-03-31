package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Printer struct {
	Expand      bool
	PrintAsJson bool
}

func (p *Printer) printAsJson(structInfo *StructInfo) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(*structInfo)
}

func (p *Printer) printAsText(structInfo *StructInfo) {
	fmt.Println(structInfo.Text(p.Expand))
}

func (p *Printer) Print(structInfo *StructInfo) {
	if p.PrintAsJson {
		p.printAsJson(structInfo)
	} else {
		p.printAsText(structInfo)
	}
}
