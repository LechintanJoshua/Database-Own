package internal

import (
	"fmt"
	"os"
	"path"
	"syscall"
)

type KV struct {
	Path string // numele fisierului
	// intern
	fd   int
	tree BTree
	// mai mult
}

// Open deschide fisierul din adresa Path
// si verifica daca aceasta a avut succes
func (db *KV) Open() error {
	fd, err := createFileSync(db.Path)

	if err != nil {
		return err
	}

	db.fd = fd

	return nil
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
// (two-phase-update)
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

// createFileSync creaza un fisier nou,
// seteaza masca la -rw-r--r-- si sincronizeaza
// directorul in care se afla
func createFileSync(file string) (int, error) {
	// obtine file descriptorul directorului
	flags := os.O_RDONLY | syscall.O_DIRECTORY
	dirfd, err := syscall.Open(path.Dir(file), flags, 0o644)

	if err != nil {
		return -1, fmt.Errorf("open director: %w", err)
	}

	defer syscall.Close(dirfd)

	// deschide sau creaza fisierul
	flags = os.O_RDWR | os.O_CREATE
	fd, err := syscall.Openat(dirfd, path.Base(file), flags, 0o644)

	if err != nil {
		return -1, fmt.Errorf("open file: %w", err)
	}

	// fsync directorul

	if err = syscall.Fsync(dirfd); err != nil {
		_ = syscall.Close(fd) // s-ar putea sa lase un fisier gol
		return -1, fmt.Errorf("fsync directory: %w", err)
	}

	return fd, nil
}
