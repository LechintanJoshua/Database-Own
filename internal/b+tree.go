package internal

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
	assert(node1max <= BTREE_PAGE_SIZE)
}

type BNode []byte

type BTree struct {
	root uint64

	get func(uint64) []byte
	new func([]byte) uint64
	del func(uint64)
}

const (
	BNODE_NODE = 1
	BNODE_LEAF = 2
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
// urmatorii 2 sunt numarul de chei
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
	new.setPtr(idx, ptr)
	pos := new.kvPos(idx)
	binary.LittleEndian.PutUint16(new[pos+0:], uint16(len(key)))
	binary.LittleEndian.PutUint16(new[pos+2:], uint16(len(val)))
	copy(new[pos+4:], key)
	copy(new[pos+4+uint16(len(key)):], val)

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
	}

	nodeAppendRange(new, old, idx+inc, idx+1, old.nkeys()-(idx+1))
}

// nodeSplit2 divizeaza un nod mare in 2 noduri
// care vor respecta dimensiunea paginii de memorie
func nodeSplit2(left BNode, right BNode, old BNode) {
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
		return 1, [3]BNode{old}
	}

	left := BNode(make([]byte, 2*BTREE_PAGE_SIZE))
	right := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(left, right, old)

	if left.nbytes() <= BTREE_PAGE_SIZE {
		left = left[:BTREE_PAGE_SIZE]
		return 2, [3]BNode{left, right}
	}

	leftleft := BNode(make([]byte, BTREE_PAGE_SIZE))
	middle := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(leftleft, middle, left)
	assert(leftleft.nbytes() <= BTREE_PAGE_SIZE)
	return 3, [3]BNode{leftleft, middle, right}
}

// treeInsert insereaza un KV intr-un nod, nodul rezultat
// poate fi divizat
// cel ce apeleaza functa este responsabil pentru dealocarea nodului
// si divizarea si alocarii rezultatelor
func treeInsert(tree *BTree, node BNode, key []byte, val []byte) BNode {
	new := BNode(make([]byte, 2*BTREE_PAGE_SIZE))
	idx := nodeLookupLE(node, key)

	switch node.btype() {
	case BNODE_LEAF:
		if bytes.Equal(key, node.getKey(idx)) {
			leafUpdate(new, node, idx, key, val)
		} else {
			leafInsert(new, node, idx+1, key, val)
		}
	case BNODE_NODE:
		nodeInsert(tree, new, node, idx, key, val)
	default:
		panic("bad node!")
	}

	return new
}

// leafUpdate actualizeaza perechea Key-Value dintr-un nod
// copiaza datele din vechiul nod in cel nou si updateaza
// kv de la indexul respectiv (copy-on-write)
func leafUpdate(
	new BNode, old BNode, idx uint16, key []byte, val []byte,
) {
	new.setHeader(BNODE_LEAF, old.nkeys())
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx+1, old.nkeys()-idx-1)
}

// nodeInsert insereaza recursiv un KV intr-un nod intern
// actualizeaza pointerul catre copilul modificat si
// gestioneaza split-ul daca copilul depaseste dimensiunea
// unei pagini de memorie
func nodeInsert(
	tree *BTree, new BNode, node BNode, idx uint16,
	key []byte, val []byte,
) {
	kptr := node.getPtr(idx)
	knode := treeInsert(tree, tree.get(kptr), key, val)
	nsplit, split := nodeSplit3(knode)
	tree.del(kptr)
	nodeReplaceKidN(tree, new, node, idx, split[:nsplit]...)
}

// Insert inesreaza o cheie noua sau actualizeaza
// o cheie existenta
// daca arborele este gol, creaza radacina
func (tree *BTree) Insert(key []byte, val []byte) {
	if tree.root == 0 {
		root := BNode(make([]byte, BTREE_PAGE_SIZE))
		root.setHeader(BNODE_LEAF, 2)
		nodeAppendKV(root, 0, 0, nil, nil)
		nodeAppendKV(root, 1, 0, key, val)

		tree.root = tree.new(root)
		return
	}

	node := treeInsert(tree, tree.get(tree.root), key, val)
	nsplit, split := nodeSplit3(node)
	tree.del(tree.root)

	if nsplit > 1 {
		root := BNode(make([]byte, BTREE_PAGE_SIZE))
		root.setHeader(BNODE_NODE, nsplit)

		for i, knode := range split[:nsplit] {
			ptr, key := tree.new(knode), knode.getKey(0)
			nodeAppendKV(root, uint16(i), ptr, key, nil)
		}

		tree.root = tree.new(root)
	} else {
		tree.root = tree.new(split[0])
	}
}

// shouldMerge verifica daca un nod trebuie unit cu
// fratii lui
// ea returneaza fratele si indexul acestuia relativ la nod
// cu care trebuie reunit
// am impus o limita de minima de 1/4 din dimensiunea blocului
// pentru ca un nod sa nu fie merge-uit
func shouldMerge(
	tree *BTree, node BNode, idx uint16, updated BNode,
) (int, BNode) {
	if updated.nbytes() > BTREE_PAGE_SIZE/4 {
		return 0, BNode{}
	}

	if idx > 0 {
		sibling := BNode(tree.get(node.getPtr(idx - 1)))
		merged := sibling.nbytes() + updated.nbytes() - HEADER

		if merged <= BTREE_PAGE_SIZE {
			return -1, sibling
		}
	}

	if idx+1 < node.nkeys() {
		sibling := BNode(tree.get(node.getPtr(idx + 1)))
		merged := sibling.nbytes() + updated.nbytes() - HEADER

		if merged <= BTREE_PAGE_SIZE {
			return 1, sibling
		}
	}

	return 0, BNode{}
}

// Sterge o cheie de la un anumit index dintr-un nod frunza
// updateaza nodul folosind (copy-on-write)
func leafDelete(new BNode, old BNode, idx uint16) {
	new.setHeader(BNODE_LEAF, old.nkeys()-1)
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendRange(new, old, idx, idx+1, old.nkeys()-idx-1)
}

// nodeMerge uneste doua noduri intr-un nou (copy-on-write)
// cheilei din left < cheile din right
func nodeMerge(new BNode, left BNode, right BNode) {
	assert(left.nbytes()+right.nbytes()-HEADER <= BTREE_PAGE_SIZE)
	new.setHeader(left.btype(), left.nkeys()+right.nkeys())

	nodeAppendRange(new, left, 0, 0, left.nkeys())
	nodeAppendRange(new, right, left.nkeys(), 0, right.nkeys())
}

// nodeReplace2Kid inlocuieste 2 linkuri adiacente
// dintr-un nod intern cu un singur link catre nodul
// rezultat din merge (copy-on-write)
func nodeReplace2Kid(
	new BNode, old BNode, idx uint16, ptr uint64, key []byte,
) {
	new.setHeader(BNODE_NODE, old.nkeys()-1)
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, ptr, key, nil)
	nodeAppendRange(new, old, idx+1, idx+2, old.nkeys()-idx-2)
}

// treeDelete cauta o cheie si o sterge din nodul corespunzator
// returneaza noul nod modificat (copy-on-write) sau un nod gol
// daca cheia nu a fost gasita
// cel ce apeleaza functia este responsabil pentru rebalansare
// si actualizarea pointerilor
func treeDelete(tree *BTree, node BNode, key []byte) BNode {
	idx := nodeLookupLE(node, key)

	switch node.btype() {
	case BNODE_LEAF:
		if bytes.Equal(key, node.getKey(idx)) {
			new := BNode(make([]byte, BTREE_PAGE_SIZE))
			leafDelete(new, node, idx)
			return new
		}

		return BNode{}
	case BNODE_NODE:
		return nodeDelete(tree, node, idx, key)
	default:
		panic("bad node!")
	}
}

// nodeDelete sterge recursiv o cheie dintr-un nod intern
// actualizeaza pointerul catre copilul modificat si
// gestioneaza merge-ul daca dimensiunea copilului scade
// sub cea acceptata (copy-on-write)
func nodeDelete(
	tree *BTree, node BNode, idx uint16, key []byte,
) BNode {
	kptr := node.getPtr(idx)
	updated := treeDelete(tree, tree.get(kptr), key)

	if len(updated) == 0 {
		return BNode{}
	}

	new := BNode(make([]byte, BTREE_PAGE_SIZE))
	mergeDir, sibling := shouldMerge(tree, node, idx, updated)

	switch {
	case mergeDir < 0:
		merged := BNode(make([]byte, BTREE_PAGE_SIZE))
		nodeMerge(merged, sibling, updated)
		tree.del(node.getPtr(idx - 1))
		nodeReplace2Kid(new, node, idx-1, tree.new(merged), merged.getKey(0))
	case mergeDir > 0:
		merged := BNode(make([]byte, BTREE_PAGE_SIZE))
		nodeMerge(merged, updated, sibling)
		tree.del(node.getPtr(idx + 1))
		nodeReplace2Kid(new, node, idx, tree.new(merged), merged.getKey(0))
	case mergeDir == 0 && updated.nkeys() == 0:
		assert(node.nkeys() == 1 && idx == 0)
		new.setHeader(BNODE_NODE, 0)
	case mergeDir == 0 && updated.nkeys() > 0:
		nodeReplaceKidN(tree, new, node, idx, updated)
	}

	return new
}

// Delete sterge o cheie din arbore
// daca cheia exista va fi stearsa si returneaza true,
// altfe false
func (tree *BTree) Delete(key []byte) bool {
	if tree.root == 0 {
		return false
	}

	updated := treeDelete(tree, tree.get(tree.root), key)

	if len(updated) == 0 {
		return false
	}

	tree.del(tree.root)

	if updated.btype() == BNODE_NODE && updated.nkeys() == 0 {
		tree.root = updated.getPtr(0)
	} else {
		tree.root = tree.new(updated)
	}

	return true
}

// Get returneaza valoarea stocata la o cheie in arbore
// daca nu exista returneaza un array gol si false
func (tree *BTree) Get(key []byte) ([]byte, bool) {
	if tree.root == 0 {
		return []byte{}, false
	}

	curr := BNode(tree.get(tree.root))

	for curr.btype() != BNODE_LEAF {
		idx := nodeLookupLE(curr, key)
		ptr := curr.getPtr(idx)
		curr = BNode(tree.get(ptr))
	}

	idx := nodeLookupLE(curr, key)
	node_key := curr.getKey(idx)

	if bytes.Equal(key, node_key) {
		return curr.getVal(idx), true
	}

	return []byte{}, false
}
