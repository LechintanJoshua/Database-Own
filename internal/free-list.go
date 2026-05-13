package internal

import "encoding/binary"

type LNode []byte

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
