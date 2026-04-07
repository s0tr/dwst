package main

import (
	"fmt"
	"strings"
)

type BaseInfo struct {
	Name  string
	Size  int64
	Depth int
}

type Field interface {
	Text(expand bool, hex bool) string
}

type FieldInfo struct {
	BaseInfo
	Type   string
	Offset int64
}

type StructInfo struct {
	FieldInfo
	IsRoot bool
	Kind   string
	Fields []Field
}

const padding = "    "

func (f *FieldInfo) Text(expand bool, hex bool) string {
	if hex {
		return fmt.Sprintf("%s %-8s %s:%s\n",
			strings.Repeat(padding, f.Depth+2),
			fmt.Sprintf("0x%x[0x%x]", f.Offset, f.Size),
			f.Name,
			f.Type)
	} else {
		return fmt.Sprintf("%s %-8s %s:%s\n",
			strings.Repeat(padding, f.Depth+2),
			fmt.Sprintf("%d[%d]", f.Offset, f.Size),
			f.Name,
			f.Type)
	}
}

func (s *StructInfo) Text(expand bool, hex bool) string {
	var sb strings.Builder
	if s.IsRoot {
		fmt.Fprintf(&sb, "%s %s\n", s.Kind, s.Name)
		if hex {
			fmt.Fprintf(&sb, "%ssize: 0x%x\n", padding, s.Size)
		} else {
			fmt.Fprintf(&sb, "%ssize: %d\n", padding, s.Size)
		}
		fmt.Fprintf(&sb, "%smembers:\n", padding)

	} else {
		sb.WriteString(s.FieldInfo.Text(expand, hex))
	}

	if s.IsRoot || expand {
		// TODO: implement <padding> print for holes in data structures
		for _, field := range s.Fields {
			sb.WriteString(field.Text(expand, hex))
		}
	}
	return sb.String()
}
