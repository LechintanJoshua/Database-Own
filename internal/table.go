package internal

import (
	"fmt"
	"strings"
)

const (
	TYPE_BYTES = 1
	TYPE_INT64 = 2
)

// celula tabelei
type Value struct {
	Type uint32
	I64  int64
	Str  []byte
}

// randul tabelei
type Record struct {
	Cols []string
	Vals []Value
}

type TableDef struct {
	Name   string
	Types  []uint32
	Cols   []string
	PKeys  int
	Prefix uint32
}

var TDEF_TABLE = &TableDef{
	Prefix: 2,
	Name:   "@table",
	Types:  []uint32{TYPE_BYTES, TYPE_BYTES},
	Cols:   []string{"name", "def"},
	PKeys:  1,
}

var TDEF_META = &TableDef{
	Prefix: 1,
	Name:   "@meta",
	Types:  []uint32{TYPE_BYTES, TYPE_BYTES},
	Cols:   []string{"key", "value"},
	PKeys:  1,
}

// AddStr adauga un camp nou string randului din tabela
// (adauga o coloana)
func (rec *Record) AddStr(col string, val []byte) *Record {
	rec.Cols = append(rec.Cols, col)
	rec.Vals = append(rec.Vals, Value{Type: TYPE_BYTES, Str: val})
	return rec
}

// AddInt64 adauga un camp nou int64 randului din tabela
// (adauga o coloana)
func (rec *Record) AddInt64(col string, val int64) *Record {
	rec.Cols = append(rec.Cols, col)
	rec.Vals = append(rec.Vals, Value{Type: TYPE_INT64, I64: val})
	return rec
}

// Get returneaza celula dintr-un rand al tabelei
// pentru o coloana respectiva
func (rec *Record) Get(col string) *Value {
	for i, str := range rec.Cols {
		if str == col {
			return &rec.Vals[i]
		}
	}

	return nil
}

// PrintRecord afiseaza frumos coloanele si valorile unui rand
func PrintRecord(rec *Record) {
	fmt.Println("--- Rezultat ---")
	for i, colName := range rec.Cols {
		val := rec.Vals[i]

		if len(val.Str) > 0 || val.Type == TYPE_BYTES {
			fmt.Printf("%s: %s\n", colName, string(val.Str))
		} else {
			fmt.Printf("%s: %d\n", colName, val.I64)
		}
	}
	fmt.Println("----------------")
}

// PrintTableResults parcurge un scanner si afiseaza fiecare rand pe o singura linie,
// unde limit reprezinta numarul maxim de randuri din tabela afisate
func PrintTableResults(tableName string, sc *Scanner, limit int) {
	fmt.Printf("\n=== Date din tabela: %s ===\n", tableName)

	rec := &Record{}
	count := 0

	for sc.Valid() {
		if limit > 0 && count >= limit {
			break
		}

		sc.Deref(rec)

		var rowInfo []string
		for i, colName := range rec.Cols {
			val := rec.Vals[i]

			if len(val.Str) > 0 || val.Type == TYPE_BYTES {
				rowInfo = append(rowInfo, fmt.Sprintf("%s: %s", colName, string(val.Str)))
			} else {
				rowInfo = append(rowInfo, fmt.Sprintf("%s: %d", colName, val.I64))
			}
		}

		fmt.Printf("Rand %d: [ %s ]\n", count+1, strings.Join(rowInfo, " | "))

		sc.Next()
		count++
	}

	if count == 0 {
		fmt.Println("Nu s-a gasit niciun rand.")
	}
}
