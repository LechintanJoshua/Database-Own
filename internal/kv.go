package internal

import "syscall"

type KV struct {
	Path string // numele fisierului
	// intern
	fd   int
	tree BTree
	// mai mult
}

// Get apeleaza metoda interna a tree si returneaza
// un tuplu (cheie, gasit)
func (db *KV) Get(key []byte) ([]byte, bool) {
	return db.tree.Get(key)
}

// Set apeleaza emtoda interna a tree si
// actualizeaza fisierul
func (db *KV) Set(key []byte, val []byte) error {
	db.tree.Insert(key, val)
	return updateFile(db)
}

// Del apeleaza metoda interna a tree, actualizeaza fisierul
// si returneaza un tuplu (sters, erroare)
func (db *KV) Del(key []byte) (bool, error) {
	deleted := db.tree.Delete(key)
	return deleted, updateFile(db)
}

// updateFile actualizeaza fisierul si verifica erorile
// scrie nodurile in pagini, sincronizeaza, actualizeaza
// radacina si sincronizeaza din nou sa fie persistent
// (double-write)
func updateFile(db *KV) error {
	// 1. Scrie noduri
	if err := writePages(db); err != nil {
		return err
	}

	// 2. fsync pentru a forca ordinea intre 1 si 3
	if err := syscall.Fsync(db.fd); err != nil {
		return err
	}

	// 3. Actualizeaza pointerul radacinii atomic
	if err := updateRoot(db); err != nil {
		return err
	}

	// 4. fsync sa fie totul persistent

	return syscall.Fsync(db.fd)
}
