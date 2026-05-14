package internal

import "encoding/binary"

type LNode []byte

type FreeList struct {
	// callback-uri pentru gestionarea spatilui pe disc
	get func(uint64) []byte // citeste o pagina
	new func([]byte) uint64 // adauga o pagina (append)
	set func(uint64) []byte // actualizeaza o pagina existenta
	// date persistente in meta pagina
	headPage uint64 // pointer la head node-ul listei
	headSeq  uint64 // numar de secventa monotonica pentru indexul in head
	tailPage uint64
	tailSeq  uint64
	// state-ul in memory
	maxSeq uint64 // tailSeq salvata pentru prevenirea consumari itemelor noi adaugate
}

const FREE_LIST_HEADER = 8
const FREE_LIST_CAP = (BTREE_PAGE_SIZE - FREE_LIST_HEADER) / 8

// getNext returneaza pointerul catre urmatorul node
func (node LNode) getNext() uint64 {
	return binary.LittleEndian.Uint64(node[0:8])
}

// setNext seteaza pointerul catre urmatorul node
func (node LNode) setNext(next uint64) {
	binary.LittleEndian.PutUint64(node[0:8], next)
}

// getPtr returneaza pointerul de la un index
func (node LNode) getPtr(idx int) uint64 {
	assert(idx >= 0 && idx < FREE_LIST_CAP)
	pos := FREE_LIST_HEADER + 8*idx
	return binary.LittleEndian.Uint64(node[pos : pos+8])
}

// setPtr seteaza pointeru-ul la un anumit index
func (node LNode) setPtr(idx int, ptr uint64) {
	assert(idx >= 0 && idx < FREE_LIST_CAP)
	pos := FREE_LIST_HEADER + 8*idx
	binary.LittleEndian.PutUint64(node[pos:pos+8], ptr)
}

// PopHead returneaza primul ID din Free List
func (fl *FreeList) PopHead() uint64 {
	ptr, head := flPop(fl)
	if head != 0 {
		fl.PushTail(head)
	}

	return ptr
}

// PushTail adauga un ID in capatul FreeList-ului
// daca ajunge la final, incearca sa recicleze
// head-ul, daca poate devine tail si ii adauga id-ul vechi
// si verifica daca mai avea un element in
// alfel creaza un nod nou
func (fl *FreeList) PushTail(ptr uint64) {
	// adauga nodul la final
	LNode(fl.set(fl.tailPage)).setPtr(seq2idx(fl.tailSeq), ptr)
	fl.tailSeq++
	// adaiga un nou nod la final daca lista e plina
	// (nu e niciodata goala)
	if seq2idx(fl.tailSeq) == 0 {
		// incearca sa il refolosesti din capul listei
		next, head := flPop(fl) // s-ar putea sa scoata head-ul

		if next == 0 {
			// sau aloca un nod nou prin adaugare
			next = fl.new(make([]byte, BTREE_PAGE_SIZE))
		}

		// leaga-l de tail
		LNode(fl.set(fl.tailPage)).setNext(next)
		fl.tailPage = next

		// adauga si nodul head daca a fost sters
		if head != 0 {
			LNode(fl.set(fl.tailPage)).setPtr(0, head)
			fl.tailSeq++
		}
	}
}

// seq2Idx face conversia din uint64 in int
// si returneaza rezultatul % 511
func seq2idx(seq uint64) int {
	return int(seq % FREE_LIST_CAP)
}

// SetMaxSeq adauga noile id-uri adaugate
// la secventa maxima (gata pentru consumare)
func (fl *FreeList) SetMaxSeq() {
	fl.maxSeq = fl.tailSeq
}

// flPop scoate un id/item din head-ul node-ului
// sterge nodul daca este gol
func flPop(fl *FreeList) (ptr uint64, head uint64) {
	if fl.headSeq == fl.maxSeq {
		return 0, 0 // nu se poate avansa
	}

	node := LNode(fl.get(fl.headPage))
	ptr = node.getPtr(seq2idx(fl.headSeq)) // id-ul/item-ul
	fl.headSeq++

	// mutam head-ul la urmatorul nod daca a devenint gol
	if seq2idx(fl.headSeq) == 0 {
		head, fl.headPage = fl.headPage, node.getNext()
		// lasam lista sa aibe macar un node sa evitam
		// cazurile speciale
		assert(fl.headPage != 0)
	}

	return
}
