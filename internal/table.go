package internal

const (
	TYPE_BYTES = 1 // string (de bytes arbitrari)
	TYPE_INT64 = 2 // integer; 64-bit cu semn
)

// celula tabelei
type Value struct {
	Type uint32 // uniune ca sa stim ce tip de date stocam in celula
	I64  int64
	Str  []byte
}

// randul tabelei
type Record struct {
	Cols []string
	Vals []Value
}

type TableDef struct {
	// definita de utilizator
	Name  string
	Types []uint32 // tipul coloanelor
	Cols  []string // numele coloanelor
	PKeys int      // primele 'PKeys' coloane sunt cheile primare
	// cheie auto asignata de BTree ca prefix a diferitelor tabele
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
