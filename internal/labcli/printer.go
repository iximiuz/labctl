package labcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Printer prints a list of values.
type Printer[T any, U []T | map[string]T] interface {
	Print(U) error
	Flush()
}

type tablePrinter[T any, U []T | map[string]T] struct {
	kind     string
	header   []string
	rowFunc  func(T) []string
	printKey bool
	writer   *tabwriter.Writer
}

// NewSliceTablePrinter creates a new table printer.
func NewSliceTablePrinter[T any, U []T](w io.Writer, header []string, rowFunc func(T) []string) Printer[T, U] {
	return &tablePrinter[T, U]{
		kind:    "slice",
		header:  header,
		rowFunc: rowFunc,
		writer:  tabwriter.NewWriter(w, 0, 4, 2, ' ', 0),
	}
}

// NewMapTablePrinter creates a new table printer.
func NewMapTablePrinter[T any, U map[string]T](w io.Writer, header []string, rowFunc func(T) []string, printKey bool) Printer[T, U] {
	return &tablePrinter[T, U]{
		kind:     "map",
		header:   header,
		rowFunc:  rowFunc,
		printKey: printKey,
		writer:   tabwriter.NewWriter(w, 0, 4, 2, ' ', 0),
	}
}

func (p *tablePrinter[T, U]) Print(items U) error {
	p.printHeader()
	p.print(items)

	return nil
}

func (p *tablePrinter[T, U]) printHeader() {
	fmt.Fprintln(p.writer, strings.Join(p.header, "\t"))
}

func (p *tablePrinter[T, U]) print(items U) {
	switch any(items).(type) {
	case []T:
		p.printSlice(any(items).([]T))
	case map[string]T:
		p.printMap(any(items).(map[string]T))
	default:
		panic(fmt.Sprintf("unsupported print type %T", items))
	}
}

func (p *tablePrinter[T, U]) printSlice(items []T) {
	for _, item := range items {
		fields := p.rowFunc(item)

		fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
	}
}

func (p *tablePrinter[T, U]) printMap(items map[string]T) {
	for name, item := range items {
		var fields []string

		if p.printKey {
			fields = append(fields, name)
		}

		fields = append(fields, p.rowFunc(item)...)

		fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
	}
}

func (p *tablePrinter[T, U]) Flush() {
	p.writer.Flush()
}

type jsonPrinter[T any, U []T | map[string]T] struct {
	encoder *json.Encoder
}

// NewJSONPrinter outputs values as JSON.
func NewJSONPrinter[T any, U []T | map[string]T](w io.Writer) Printer[T, U] {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return &jsonPrinter[T, U]{
		encoder: encoder,
	}
}

func (p *jsonPrinter[T, U]) Print(items U) error {
	return p.encoder.Encode(items)
}

func (p *jsonPrinter[T, U]) Flush() {}

type mapKeyPrinter[T any, U map[string]T] struct {
	writer io.Writer
}

// NewMapKeyPrinter outputs the keys of a map.
func NewMapKeyPrinter[T any, U map[string]T](w io.Writer) Printer[T, U] {
	return &mapKeyPrinter[T, U]{
		writer: w,
	}
}

func (p *mapKeyPrinter[T, U]) Print(items U) error {
	for name := range items {
		fmt.Fprintln(p.writer, name)
	}

	return nil
}

func (p *mapKeyPrinter[T, U]) Flush() {}
