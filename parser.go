package main

import (
	"compress/gzip"
	"debug/dwarf"
	"debug/elf"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

type Parser struct {
	seen      map[string]struct{}
	dwData    *dwarf.Data
	fromCache bool
	path      string
}

type ProcessStructInfoCallback func(*StructInfo) error

func NewParser(path string) (*Parser, error) {
	var dwData *dwarf.Data
	fromCache, err := isGobGz(path)
	if err != nil {
		fromCache = false
	}
	if fromCache {
		gob.Register(&FieldInfo{})
		gob.Register(&StructInfo{})
		dwData = nil
	} else {
		io, err := elf.Open(opts.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s", path)
		}
		defer io.Close()

		dwData, err = io.DWARF()
		if err != nil {
			return nil, fmt.Errorf("failed to read DWARF from %s", path)
		}
	}
	return &Parser{
		seen:      make(map[string]struct{}),
		dwData:    dwData,
		fromCache: fromCache,
		path:      path,
	}, nil
}

func (p *Parser) parseField(structField *dwarf.StructField, offset int64, depth int) (Field, error) {
	typeUnwrapped := unwrapType(structField.Type)

	fieldInfo := FieldInfo{
		BaseInfo: BaseInfo{
			Name:  structField.Name,
			Size:  structField.Type.Size(),
			Depth: depth,
		},
		Type:   removeBraces(structField.Type.String()),
		Offset: structField.ByteOffset + offset,
	}

	switch t := typeUnwrapped.(type) {
	case *dwarf.StructType:
		fields, err := p.parseFields(t, fieldInfo.Offset, depth+1)
		if err != nil {
			return nil, err
		}
		return &StructInfo{
			FieldInfo: fieldInfo,
			Fields:    fields,
			IsRoot:    false,
		}, nil
	default:
		return &fieldInfo, nil
	}
}

func (p *Parser) parseFields(structType *dwarf.StructType, offset int64, depth int) ([]Field, error) {
	fields := make([]Field, 0, len(structType.Field))

	for _, field := range structType.Field {
		parsedField, err := p.parseField(field, offset, depth)
		if err != nil {
			return nil, err
		}
		fields = append(fields, parsedField)
	}
	return fields, nil
}

func (p *Parser) ParseStructInfo(offset dwarf.Offset) (*StructInfo, error) {
	// find the type by offset
	offsetType, err := p.dwData.Type(offset)
	if err != nil {
		return nil, err
	}

	// validate that it is a StructType
	structType, isStructType := offsetType.(*dwarf.StructType)
	if !isStructType {
		return nil, fmt.Errorf("object at specified offset is not a StructType")
	}

	// parse fields
	fields, err := p.parseFields(structType, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct fields for %s", structType.Name)
	}

	return &StructInfo{
		FieldInfo: FieldInfo{
			BaseInfo: BaseInfo{
				Name: structType.StructName,
				Size: structType.ByteSize,
			},
		},
		IsRoot: true,
		Kind:   structType.Kind,
		Fields: fields,
	}, nil
}

func (p *Parser) reset() {
	p.seen = make(map[string]struct{})
}

func (p *Parser) IterateStructInfoWithCallbackFromCache(processStructInfo ProcessStructInfoCallback) error {
	f, err := os.Open(p.path)
	if err != nil {
		return fmt.Errorf("failed to open cache file %s", p.path)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to read cache file %s", p.path)
	}
	defer gz.Close()

	dec := gob.NewDecoder(gz)
	for {
		var s StructInfo
		err := dec.Decode(&s)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to decode struct")
		}
		if err = processStructInfo(&s); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) IterateStructInfoWithCallbackFromDwarf(processStructInfo ProcessStructInfoCallback) error {
	p.reset()
	reader := p.dwData.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			return err
		}
		if entry == nil {
			break
		}

		if entry.Tag == dwarf.TagStructType || entry.Tag == dwarf.TagUnionType {
			if !entry.Children {
				continue
			}
			name, _ := entry.Val(dwarf.AttrName).(string)
			if name == "" {
				continue
			}
			// check the hash
			t, err := p.dwData.Type(entry.Offset)
			if err != nil {
				return err
			}
			st, ok := t.(*dwarf.StructType)
			if !ok {
				continue
			}
			if st.StructName == "" {
				continue
			}
			key := structKey(st)
			if _, seen := p.seen[key]; seen {
				continue
			}
			// parse new struct
			p.seen[key] = struct{}{}
			structInfo, err := p.ParseStructInfo(entry.Offset)
			if err != nil {
				return err
			}
			err = processStructInfo(structInfo)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Parser) IterateStructInfoWithCallback(processStructInfo ProcessStructInfoCallback) error {
	if p.fromCache {
		return p.IterateStructInfoWithCallbackFromCache(processStructInfo)
	}
	return p.IterateStructInfoWithCallbackFromDwarf(processStructInfo)
}

func (p *Parser) cacheStructsInternal(cachePath string) (ProcessStructInfoCallback, func(), error) {
	f, err := os.Create(cachePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cache file %s", cachePath)
	}

	gz := gzip.NewWriter(f)
	enc := gob.NewEncoder(gz)

	callback := func(s *StructInfo) error {
		return enc.Encode(s)
	}

	closer := func() {
		gz.Close()
		f.Close()
	}
	return callback, closer, nil
}

func (p *Parser) CacheStructs(cachePath string) error {
	gob.Register(&FieldInfo{})
	gob.Register(&StructInfo{})
	callback, closer, err := p.cacheStructsInternal(cachePath)
	if err != nil {
		return err
	}
	defer closer()
	return p.IterateStructInfoWithCallback(callback)
}

func structKey(st *dwarf.StructType) string {
	hash := crc32.NewIEEE()
	fmt.Fprintf(hash, "%s:%d", st.StructName, st.ByteSize)
	for _, f := range st.Field {
		fmt.Fprintf(hash, ":%s:%s:%d", f.Name, f.Type, f.ByteOffset)
	}
	return fmt.Sprintf("%x", hash.Sum32())
}

func unwrapType(t dwarf.Type) dwarf.Type {
	switch v := t.(type) {
	case *dwarf.QualType:
		return unwrapType(v.Type)
	case *dwarf.TypedefType:
		return unwrapType(v.Type)
	case *dwarf.ArrayType:
		return unwrapType(v.Type)
	default:
		return t
	}
}

func isGobGz(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer f.Close()

	magic := make([]byte, 2)
	_, err = f.Read(magic)
	if err != nil {
		return false, err
	}
	return magic[0] == 0x1f && magic[1] == 0x8b, nil
}

func removeBraces(s string) string {
	result := make([]rune, 0, len(s))
	var depth int

	for _, ch := range s {
		switch ch {
		case '{':
			// enums and structs have one ' ' before {
			if depth == 0 && len(result) > 0 && result[len(result)-1] == ' ' {
				result = result[:len(result)-1]
			}
			depth++
			continue
		case '}':
			if depth > 0 {
				depth--
			}
			continue
		default:
			if depth == 0 {
				result = append(result, ch)
			}
		}
	}
	return string(result)
}
