package pkg

import (
	"bytes"
	"encoding/binary"
)

const HEADER = 4

const BTREE_PAGE_SIZE = 4096
const BTREE_MAX_KEY_SIZE = 1000
const BTREE_MAX_VAL_SIZE = 3000

// init verifica ca maximul unui nod sa nu depaseasca
// maximul unui bloc de memorie
func init() {
	node1max := HEADER + 8 + 2 + 4 + BTREE_MAX_KEY_SIZE + BTREE_MAX_VAL_SIZE
	assert(node1max <= BTREE_PAGE_SIZE) // maximul pentru key-value
}

type BNode []byte

type BTree struct {
	root uint64

	//callback-uri pentru managementul paginilor de pe disc
	get func(uint64) []byte //dereferentierea unui pointer
	new func([]byte) uint64 //alocarea unei noi pagini
	del func(uint64)        // dealocarea unei pagini
}

const (
	BNODE_NODE = 1 //Noduri interne fara valori
	BNODE_LEAF = 2 //Noduri frunza cu valori
)

// btype citeste din blocul de memorie pentru a afla daca
// nodul este frunza sau intern
func (node BNode) btype() uint16 {
	return binary.LittleEndian.Uint16(node[0:2])
}

// nkeys citeste din blocul de memorie pentru a afla
// numarul de chei ale nodului
func (node BNode) nkeys() uint16 {
	return binary.LittleEndian.Uint16(node[2:4])
}

// setHeader seteaza primi 4 octeti ai blocului de memorie
// primi 2 sunt tipul nodului
// urmatorii 2 sunt numarul de
func (node BNode) setHeader(btype, nkeys uint16) {
	binary.LittleEndian.PutUint16(node[0:2], btype)
	binary.LittleEndian.PutUint16(node[2:4], nkeys)
}

// getPtr returneaza adresa pointerului de la un anumit index
func (node BNode) getPtr(idx uint16) uint64 {
	assert(idx < node.nkeys())
	pos := HEADER + 8*idx
	return binary.LittleEndian.Uint64(node[pos:])
}

// setPtr seteaza adresa unui pointer de la un anumit index
func (node BNode) setPtr(idx uint16, val uint64) {
	assert(idx < node.nkeys())
	pos := HEADER + 8*idx
	binary.LittleEndian.PutUint64(node[pos:], val)
}

// offsetPos returneaza positia/indexul offsetului
func offsetPos(node BNode, idx uint16) uint16 {
	assert(1 <= idx && idx <= node.nkeys())
	return HEADER + 8*node.nkeys() + 2*(idx-1)
}

// getOffset returneaza valoarea stocata in offset pentru
// a parcurge blocul de memorie
func (node BNode) getOffset(idx uint16) uint16 {
	if idx == 0 {
		return 0
	}

	return binary.LittleEndian.Uint16(node[offsetPos(node, idx):])
}

// setOffset seteaza offsetul de la un anumit index
func (node BNode) setOffset(idx uint16, offset uint16) {
	assert(1 <= idx && idx <= node.nkeys())

	pos := HEADER + 8*node.nkeys() + 2*(idx-1)
	binary.LittleEndian.PutUint16(node[pos:], offset)
}

// kvPos calculeaza si returneaza indexul unde se afla
// stocate date unui nod (key-value)
func (node BNode) kvPos(idx uint16) uint16 {
	assert(idx <= node.nkeys())
	return HEADER + 8*node.nkeys() + 2*node.nkeys() + node.getOffset(idx)
}

// getKey returneaza cheia unei valori de la un index din nod
func (node BNode) getKey(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos:])
	return node[pos+4:][:klen]
}

// getVal returneaza valoarea de la un index din nod
func (node BNode) getVal(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos:])
	vlen := binary.LittleEndian.Uint16(node[pos+2:])
	return node[pos+4+klen:][:vlen]
}

// returneaza ultimul index, in bytes, al nodului
// (sfarsitul spatilui de memorie ocupat)
func (node BNode) nbytes() uint16 {
	return node.kvPos(node.nkeys())
}

// return the first kid node whose range intersects the key (kid[i] <= key)
// TODO: binary search
// nodeLookupLE parcurge indexii noduri si verifica valoarea
// cu cheia data ca parametru pana cand aceasta este <=
func nodeLookupLE(node BNode, key []byte) uint16 {
	nkeys := node.nkeys()
	found := uint16(0)

	// prima cheie este copia de la nodul parinte\
	// deci mereu mai mica sau egala cu cheia cautata

	for i := uint16(1); i < nkeys; i++ {
		cmp := bytes.Compare(node.getKey(i), key)

		if cmp <= 0 {
			found = i
		}

		if cmp >= 0 {
			break
		}
	}

	return found
}

// leafInsert copiaza datele din vechiul nod in cel nou,
// si introduce cheia si val la locul ei (copy-on-write)
func leafInsert(
	new BNode, old BNode, idx uint16,
	key []byte, val []byte,
) {
	new.setHeader(BNODE_LEAF, old.nkeys()+1) //setarea header-ului
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx, old.nkeys()-idx)
}

// nodeAppendKV adauga in nod la un anumit index o noua pereche
// KV impreuna cu pointerul
func nodeAppendKV(new BNode, idx uint16, ptr uint64, key []byte, val []byte) {
	// pointeri
	new.setPtr(idx, ptr)
	//KV
	pos := new.kvPos(idx)
	binary.LittleEndian.PutUint16(new[pos+0:], uint16(len(key)))
	binary.LittleEndian.PutUint16(new[pos+2:], uint16(len(val)))
	copy(new[pos+4:], key)
	copy(new[pos+4+uint16(len(key)):], val)

	//offsetul cheii urmatoare
	new.setOffset(idx+1, new.getOffset(idx)+4+uint16(len(key)+len(val)))
}

// copieaza multiple kv-uri in pozitie de la nodul vechi
func nodeAppendRange(
	new BNode, old BNode,
	dstNew uint16, srcOld uint16, n uint16,
) {
	for i := uint16(0); i < n; i++ {
		key := old.getKey(srcOld + i)
		val := old.getVal(srcOld + i)
		ptr := old.getPtr(srcOld + i)

		nodeAppendKV(new, dstNew+i, ptr, key, val)
	}
}

// schimbarea unui link cu 1 sau mai multe linkuri
func nodeReplaceKidN(
	tree *BTree, new BNode, old BNode, idx uint16,
	kids ...BNode,
) {
	inc := uint16(len(kids))
	new.setHeader(BNODE_NODE, old.nkeys()+inc-1)
	nodeAppendRange(new, old, 0, 0, idx)

	for i, node := range kids {
		nodeAppendKV(new, idx+uint16(i), tree.new(node), node.getKey(0), nil)
		// 					^pozitie		^pointer		^cheie			^val
	}

	nodeAppendRange(new, old, idx+inc, idx+1, old.nkeys()-(idx+1))
}
