package internal

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"unsafe"
)

// Structura simuleaza un disc direct in memoria ram
type C struct {
	tree  BTree
	ref   map[string]string
	pages map[uint64]BNode
}

// newC initializeaza datele structurii si
// creaza metodele pentru simulare ale callbackurilor
func newC() *C {
	pages := map[uint64]BNode{}

	return &C{
		tree: BTree{
			get: func(ptr uint64) []byte {
				node, ok := pages[ptr]
				assert(ok)
				return node
			},
			new: func(node []byte) uint64 {
				assert(BNode(node).nbytes() <= BTREE_PAGE_SIZE)
				ptr := uint64(uintptr(unsafe.Pointer(&node[0])))
				assert(pages[ptr] == nil)
				pages[ptr] = node
				return ptr
			},
			del: func(ptr uint64) {
				assert(pages[ptr] != nil)
				delete(pages, ptr)
			},
		},
		ref:   map[string]string{},
		pages: pages,
	}
}

// add adauga o cheie si o valoare in arbore si
// in referinta structurii
func (c *C) add(key string, val string) {
	c.tree.Insert([]byte(key), []byte(val))
	c.ref[key] = val
}

// TestBTreeBasic creaza o structura noua,
// verifica inserarea prin adaugare a numerelor aleatore
// sterge jumatate din arbore pentru verificare a stergerii
// si verifica simularea disscului sa fie actualizata corespunzator
func TestBTreeBasic(t *testing.T) {
	test_struct := newC()

	for range 10000 {
		k := fmt.Sprintf("key_%05d", rand.Intn(100000))
		v := fmt.Sprintf("value_%05d", rand.Intn(100000))
		test_struct.add(k, v)
	}

	for key, value := range test_struct.ref {
		node_value, ok := test_struct.tree.Get([]byte(key))

		if !ok {
			t.Fatalf("Cheia %s nu exista in arbore. Asteptat: %s", key, value)
		}

		if !bytes.Equal(node_value, []byte(value)) {
			t.Fatalf("Valoare gresita la cheia %s. Asteptat: %s, Primit: %s",
				key, value, string(node_value))
		}
	}

	deleted := 0

	for key := range test_struct.ref {
		if deleted == 5000 {
			break
		}

		ok := test_struct.tree.Delete([]byte(key))
		delete(test_struct.ref, key)

		if !ok {
			t.Fatalf("Cheia %s nu a fost stearsa din arbore", key)
		}

		deleted += 1
	}

	for key, value := range test_struct.ref {
		node_value, ok := test_struct.tree.Get([]byte(key))

		if !ok {
			t.Fatalf("Cheia %s nu exista in arbore. Asteptat: %s", key, value)
		}

		if !bytes.Equal(node_value, []byte(value)) {
			t.Fatalf("Valoare gresita la cheia %s. Asteptat: %s, Primit: %s",
				key, value, string(node_value))
		}
	}
}
