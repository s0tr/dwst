package main

import (
	"strings"
)

type CompareFieldFunc func(*FieldInfo) bool

type Filter struct {
	Exact bool

	SizeLow  int64
	SizeHigh int64

	FieldOffset int64
	FieldName   string

	StructName string
}

func (f *Filter) stringCompare(str1 string, str2 string) bool {
	if f.Exact {
		return str1 == str2
	}
	return strings.Contains(str1, str2)
}

func (f *Filter) matchSize(structInfo *StructInfo) bool {
	if f.SizeLow != -1 && f.SizeHigh != -1 {
		return structInfo.Size >= f.SizeLow && structInfo.Size <= f.SizeHigh
	}
	if f.SizeLow == -1 && f.SizeHigh != -1 {
		return structInfo.Size <= f.SizeHigh
	}
	if f.SizeLow != -1 && f.SizeHigh == -1 {
		return structInfo.Size >= f.SizeLow
	}
	return true
}

func (f *Filter) matchName(structInfo *StructInfo) bool {
	if f.StructName == "" {
		return true
	}
	return f.stringCompare(structInfo.Name, f.StructName)
}

func (f *Filter) matchFields(fields []Field, compareFunc CompareFieldFunc) bool {
	for _, field := range fields {
		switch v := field.(type) {
		case *StructInfo:
			if compareFunc(&v.FieldInfo) {
				return true
			}
			if f.matchFields(v.Fields, compareFunc) {
				return true
			}
		case *FieldInfo:
			if compareFunc(v) {
				return true
			}
		}
	}
	return false
}

func (f *Filter) compareTypeOffset(field *FieldInfo) bool {
	return field.Offset == f.FieldOffset && f.stringCompare(field.Type, f.FieldName)
}

func (f *Filter) matchTypeOffset(structInfo *StructInfo) bool {
	if f.FieldOffset == -1 || f.FieldName == "" {
		return true
	}
	return f.matchFields(structInfo.Fields, f.compareTypeOffset)
}

func (f *Filter) Match(structInfo *StructInfo) bool {
	return f.matchSize(structInfo) && f.matchName(structInfo) && f.matchTypeOffset(structInfo)
}
