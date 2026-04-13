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
// nodeLookupLE parcurge indexii noduri si verifica valoarea
// cu cheia data ca parametru pana cand aceasta este <=
func nodeLookupLE(node BNode, key []byte) uint16 {
	nkeys := node.nkeys() - 1
	fskey := uint16(1)
	found := uint16(0)

	for fskey <= nkeys {
		mid := fskey + (nkeys-fskey)/2
		cmp := bytes.Compare(node.getKey(mid), key)

		if cmp <= 0 {
			found = mid
			fskey = mid + 1
		} else {
			nkeys = mid - 1
		}

		if cmp == 0 {
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

// nodeAppendRange copiaza in noul nod, datele de la vechiul nod
// incepand de la un index dstNew (pentru noul nod) si incepand
// de la srcOld (pentru vechiul nod)
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

// nodeReplaceKidN schimba linkurile unui nod divizat
// cu 1 sau mai multe
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

// nodeSplit2 divizeaza un nod mare in 2 noduri
// care vor respecta dimensiunea paginii de memorie
func nodeSplit2(left BNode, right BNode, old BNode) {
	// gaseste indexul din mijloc al nodului old si
	// divizeazal in doua
	mid := old.nkeys() / 2
	left.setHeader(old.btype(), mid)
	right.setHeader(old.btype(), old.nkeys()-mid)

	nodeAppendRange(left, old, 0, 0, mid)
	nodeAppendRange(right, old, 0, mid, old.nkeys()-mid)
}

// nodeSplit3 verifica dimensiunea unui nod si il divizeaza
// corespunzator.
// Deloc daca este in limita, in 2, sau in 3
func nodeSplit3(old BNode) (uint16, [3]BNode) {
	if old.nbytes() <= BTREE_PAGE_SIZE {
		old = old[:BTREE_PAGE_SIZE]
		return 1, [3]BNode{old} // ne-divizat
	}

	left := BNode(make([]byte, 2*BTREE_PAGE_SIZE)) //poate fi divizat mai tarziu
	right := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(left, right, old)

	if left.nbytes() <= BTREE_PAGE_SIZE {
		left = left[:BTREE_PAGE_SIZE]
		return 2, [3]BNode{left, right} // 2 noduri
	}

	leftleft := BNode(make([]byte, BTREE_PAGE_SIZE))
	middle := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(leftleft, middle, left)
	assert(leftleft.nbytes() <= BTREE_PAGE_SIZE)
	return 3, [3]BNode{leftleft, middle, right} // 3 noduri
}

// // insereaza un KV intr-un nod, nodul rezultat
// // poate fi divizat
// // cel ce apeleaza functia este responsabil pentru
// // dealocarea nodului si divizarea si alocarii
// // rezultatelor

// func treeInsert(tree *BTree, node BNode, key []byte, val []byte) BNode {
// 	// rezultatul este un nod
// 	// are voie sa fie mai mare decat o pagina
// 	// (oricum va fi divizat)

// 	new := BNode{data: make([]byte, 2*BTREE_PAGE_SIZE)}

// 	// unde sa insereze cheia?

// 	idx := nodeLookupLE(node, key)

// 	// actioneaza in functie de tipul nodului

// 	switch node.btype() {
// 	case BNODE_LEAF:
// 		// frunza, node.getKey(idx) <= key
// 		if bytes.Equal(key, node.getKey(idx)) {

// 		}
// 	}
// }
