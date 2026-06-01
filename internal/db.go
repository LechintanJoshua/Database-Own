package internal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

type DB struct {
	Path string
	kv   KV
}

// Scanner este wrapper peste BIterator
type Scanner struct {
	// range-ul, de la cheia 1 la cheia 2
	Cmp1   int // CMP_?
	Cmp2   int
	Key1   Record
	Key2   Record
	iter   *BIter
	tdef   *TableDef
	keyEnd []byte
}

// Get cauta definitia tabelei si daca aceasta exista
// apoi obtine un singur rand
func (db *DB) Get(table string, rec *Record) (bool, error) {
	tdef := getTableDef(db, table)
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}

	return dbGet(db, tdef, rec)
}

// Insert verifica daca tabela exista, si insereaca un rand in ea
func (db *DB) Insert(table string, rec Record) (bool, error) {
	tdef := getTableDef(db, table)
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}

	return dbUpdate(db, tdef, rec, MODE_INSERT_ONLY)
}

// Update verifica daca tabela exista si actualizeaza un rand in ea
func (db *DB) Update(table string, rec Record) (bool, error) {
	tdef := getTableDef(db, table)
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}

	return dbUpdate(db, tdef, rec, MODE_UPDATE_ONLY)
}

// Upsert verifica daca tabela exista si actualizeaza sau scrie
// un rand in ea
func (db *DB) Upsert(table string, rec Record) (bool, error) {
	tdef := getTableDef(db, table)
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}
	return dbUpdate(db, tdef, rec, MODE_UPSERT)
}

// Delete verifica daca tabela exista si sterge un rand din ea
func (db *DB) Delete(table string, rec Record) (bool, error) {
	tdef := getTableDef(db, table)
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}

	return dbDelete(db, tdef, rec)
}

// dbGet obtine un rand dintr-un tabel pe baza chei primare,
// populeaza structura rec si returneaza un tuplu
// pentru existensa sa si eroare
func dbGet(db *DB, tdef *TableDef, rec *Record) (bool, error) {
	values, err := checkRecord(tdef, *rec, tdef.PKeys)
	if err != nil {
		return false, err
	}

	key := encodeKey(nil, tdef.Prefix, values[:tdef.PKeys])
	val, ok := db.kv.Get(key)
	if !ok {
		return false, nil
	}

	for i := tdef.PKeys; i < len(tdef.Cols); i++ {
		values[i].Type = tdef.Types[i]
	}

	decodeValues(val, values[tdef.PKeys:])
	rec.Cols = tdef.Cols
	rec.Vals = values
	return true, nil
}

// checkRecord rearanjeaza ordineal coloanelor date de urilizator
// in functie de ordinea coloanelor din definita tabelului
// si verifica ca acestea sa existe sau sa respecte tipul
// valorilor stocate in ele
// n == tdef.Pkeys: inseamna ca randul este exact o cheie primara
// n == len(tdef.Cols): randul contine toate coloanele
func checkRecord(tdef *TableDef, rec Record, n int) ([]Value, error) {
	values := make([]Value, n)
	var err error

	for i := range n {
		check := rec.Get(tdef.Cols[i])

		if check == nil {
			return nil, fmt.Errorf("missing column; %s", tdef.Cols[i])
		}

		if check.Type != tdef.Types[i] {
			return nil, fmt.Errorf("bad type for column: %s", tdef.Cols[i])
		}

		values[i] = *check
	}

	return values, err
}

// encodeKey codifica coloanele pentru a afla cheia lor in arbore
func encodeKey(out []byte, prefix uint32, vals []Value) []byte {
	prefBuf := make([]byte, 4)
	size := make([]byte, 4)
	buff := make([]byte, 8)
	binary.LittleEndian.PutUint32(prefBuf[:], prefix)

	out = append(out, prefBuf...)

	for _, v := range vals {
		switch v.Type {
		case TYPE_BYTES:
			binary.LittleEndian.PutUint32(size[:], uint32(len(v.Str)))
			out = append(out, size...)
			out = append(out, v.Str...)
		case TYPE_INT64:
			binary.LittleEndian.PutUint64(buff[:], uint64(v.I64))
			out = append(out, buff...)
		}
	}

	return out
}

// encodeValues codifica valorile ramase dupa cheia primara
func encodeValues(out []byte, vals []Value) []byte {
	sizeBuf := make([]byte, 4)
	buff := make([]byte, 8)

	for _, v := range vals {
		switch v.Type {
		case TYPE_BYTES:
			binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(v.Str)))
			out = append(out, sizeBuf...)
			out = append(out, v.Str...)
		case TYPE_INT64:
			binary.LittleEndian.PutUint64(buff[:], uint64(v.I64))
			out = append(out, buff...)
		}
	}

	return out
}

// decodeBytes este o functie ajutatoare pentru a decodifica
// valorile de tip byte de pe disc
// decodifica in functie de escape bytes, 0x00, 0x01, 0x02
func decodeBytes(in []byte, out []byte, idx uint32) ([]byte, uint32) {
	for idx < uint32(len(in)) {
		b := in[idx]

		switch b {
		case 0x00:
			idx++
			return out, idx
		case 0x01:
			nextB := in[idx+1]
			if nextB == 0x01 {
				out = append(out, 0x00)
			}
			if nextB == 0x02 {
				out = append(out, 0x01)
			}
			idx += 2
		default:
			out = append(out, b)
			idx++

		}
	}

	return out, idx
}

// decodeKeys decodifica chiele primite de pe disc
// in tipul Value
func decodeKeys(in []byte, out []Value) {
	idx := uint32(4)
	var s []byte

	for i, v := range out {
		switch v.Type {
		case TYPE_BYTES:
			s = []byte{}
			s, idx = decodeBytes(in, s, idx)
			out[i].Str = s
		case TYPE_INT64:
			// schimbare bit semn
			u := int64(binary.BigEndian.Uint64(in[idx:]) - (1 << 63))
			out[i].I64 = u
			idx += 8
		default:
			panic("bad type")
		}
	}
}

// decodeValues decodifica valorile primite de pe
// disc in tipul Value
func decodeValues(in []byte, out []Value) {
	idx := uint32(0)

	for i, v := range out {
		switch v.Type {
		case TYPE_BYTES:
			size := binary.LittleEndian.Uint32(in[idx : idx+4])
			idx += 4
			out[i].Str = in[idx : idx+size]
			idx += size
		case TYPE_INT64:
			out[i].I64 = int64(binary.LittleEndian.Uint64(in[idx : idx+8]))
			idx += 8
		}
	}
}

// getTableDef cauta si verifica existenta definitiei
// tabelei pe disc (dupa nume), o aduce in memorie,
// o parseaza si creaza o noua structura cu definitia
// exacta a tabelei
func getTableDef(db *DB, name string) *TableDef {
	rec := (&Record{}).AddStr("name", []byte(name))
	ok, err := dbGet(db, TDEF_TABLE, rec)
	assert(err == nil)
	if !ok {
		return nil
	}

	tdef := &TableDef{}
	err = json.Unmarshal(rec.Get("def").Str, tdef)
	assert(err == nil)
	return tdef
}

// dbUpdate actualizeaza o tabela in trei moduri
// insert, update sau upsert
func dbUpdate(db *DB, tdef *TableDef, rec Record, mode int) (bool, error) {
	values, err := checkRecord(tdef, rec, len(tdef.Cols))
	if err != nil {
		return false, err
	}
	key := encodeKey(nil, tdef.Prefix, values[:tdef.PKeys])
	val := encodeValues(nil, values[tdef.PKeys:])
	return db.kv.Update(key, val, mode)
}

// TableNew verifica daca tabela exista deja
// altfel creaza o tabela noua
// incrementeaza contorul in @meta si adauga
// schema la @table
func (db *DB) TableNew(tdef *TableDef) error {
	if getTableDef(db, tdef.Name) != nil {
		return fmt.Errorf("table already exists: %s", tdef.Name)
	}

	rec := (&Record{}).AddStr("key", []byte("next_prefix"))
	ok, err := dbGet(db, TDEF_META, rec)
	var nextPrefix uint32

	if err != nil {
		return err
	}

	if !ok {
		nextPrefix = 3
		rec.AddStr("value", make([]byte, 4))
	} else {
		nextPrefix = binary.LittleEndian.Uint32(rec.Get("value").Str[:])
	}

	tdef.Prefix = nextPrefix
	nextPrefix++
	binary.LittleEndian.PutUint32(rec.Get("value").Str[:], nextPrefix)

	_, err = dbUpdate(db, TDEF_META, *rec, MODE_UPSERT)

	if err != nil {
		return err
	}

	value, err := json.Marshal(tdef)

	if err != nil {
		return err
	}

	rec = (&Record{}).AddStr("name", []byte(tdef.Name))
	rec.AddStr("def", value)

	_, err = dbUpdate(db, TDEF_TABLE, *rec, MODE_UPSERT)

	if err != nil {
		return err
	}

	return nil
}

// dbDelete sterge un rand dintr-o tabela
func dbDelete(db *DB, tdef *TableDef, rec Record) (bool, error) {
	values, err := checkRecord(tdef, rec, tdef.PKeys)
	if err != nil {
		return false, err
	}
	key := encodeKey(nil, tdef.Prefix, values[:tdef.PKeys])
	return db.kv.Del(key)
}

// Valid verifica daca suntem in range-ul dat
func (sc *Scanner) Valid() bool {
	if !sc.iter.Valid() {
		return false
	}

	var cond bool
	key, _ := sc.iter.Deref()
	cmp := bytes.Compare(key, sc.keyEnd)

	switch sc.Cmp2 {
	case CMP_GE:
		cond = cmp >= 0
	case CMP_GT:
		cond = cmp > 0
	case CMP_LE:
		cond = cmp <= 0
	case CMP_LT:
		cond = cmp < 0
	default:
		panic("bad compare")
	}

	return cond
}

// Next muta iteratorul
func (sc *Scanner) Next() {
	sc.iter.Next()
}

// Deref aduce randul curent si il salveaza in record
func (sc *Scanner) Deref(rec *Record) {
	key, val := sc.iter.Deref()
	rec.Cols = sc.tdef.Cols
	data := make([]Value, len(sc.tdef.Cols))
	for i, t := range sc.tdef.Types {
		data[i].Type = t
	}
	decodeKeys(key, data[:sc.tdef.PKeys])
	decodeValues(val, data[sc.tdef.PKeys:])
	rec.Vals = data
}

// Scan obtine definitia tabelei de pe disc si
// porneste iteratorul de la prima cheie
func (db *DB) Scan(table string, req *Scanner) error {
	var err error
	req.tdef = getTableDef(db, table)
	if req.tdef == nil {
		return fmt.Errorf("table not found: %s", table)
	}

	req.Key1.Vals, err = checkRecord(req.tdef, req.Key1, req.tdef.PKeys)
	if err != nil {
		return err
	}

	req.Key2.Vals, err = checkRecord(req.tdef, req.Key2, req.tdef.PKeys)
	if err != nil {
		return err
	}

	keyStart := encodeKey(nil, req.tdef.Prefix, req.Key1.Vals[:req.tdef.PKeys])
	req.keyEnd = encodeKey(nil, req.tdef.Prefix, req.Key2.Vals[:req.tdef.PKeys])
	req.iter = db.kv.tree.Seek(keyStart, req.Cmp1)

	return nil
}
