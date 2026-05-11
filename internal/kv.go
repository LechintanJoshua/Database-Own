package internal

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

type KV struct {
	Path string // numele fisierului
	// intern
	fd   int
	tree BTree

	// mai mult

	mmap struct {
		total  int      // mmap size, poate sa fie mai mare decat file size
		chunks [][]byte // mmaps multiple, pot sa nu fie continue
	}

	page struct {
		flushed uint64   // marimea bazei de date in nr de pagini
		temp    [][]byte // paginile noi alocate
	}
}

// Open deschide fisierul din adresa Path
// si verifica daca aceasta a avut succes
func (db *KV) Open() error {
	db.tree.get = db.pageRead   // read a page
	db.tree.new = db.pageAppend // append a page
	db.tree.del = func(uint64) {}

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

// pageRead citeste o pagina din memoria unui fisier
// si returneaza nodul asociat acelei pagini
func (db *KV) pageRead(ptr uint64) []byte {
	start := uint64(0)

	for _, chunk := range db.mmap.chunks {
		end := start + uint64(len(chunk))/BTREE_PAGE_SIZE

		if ptr < end {
			offset := BTREE_PAGE_SIZE * (ptr - start)

			return chunk[offset : offset+BTREE_PAGE_SIZE]
		}

		start = end
	}

	panic("bad ptr")
}

// extendMmap dubleaza spatiul de memorie din fisier
// si adauga o noua pagina in chunk
func extendMmap(db *KV, size int) error {
	if size <= db.mmap.total {
		return nil
	}

	// 64 << 20 (64 megabytes)
	alloc := max(db.mmap.total, 64<<20) // dubleaza spatiul curent de la adresa

	for db.mmap.total+alloc < size {
		alloc *= 2
	}

	chunk, err := syscall.Mmap(
		db.fd, int64(db.mmap.total), alloc,
		syscall.PROT_READ, syscall.MAP_SHARED, // doar citire
	)

	if err != nil {
		return fmt.Errorf("mmap: %w", err)
	}

	db.mmap.total += alloc
	db.mmap.chunks = append(db.mmap.chunks, chunk)

	return nil
}

// adauga un nod la o noua pagina in memore (RAM)
// functia face adaugarea append only
// returnam ptr pentru inceputul nodului in pagina
func (db *KV) pageAppend(node []byte) uint64 {
	ptr := db.page.flushed + uint64(len(db.page.temp))
	db.page.temp = append(db.page.temp, node)

	return ptr
}

// writePages scrie paginile cu noduri stocate temporar
// in fisierul de pe disc si verifica de erori
// actualizeaza numarul paginilor totale si reseteaza temp
func writePages(db *KV) error {
	// extinde mmap daca trebuie
	size := (int(db.page.flushed) + len(db.page.temp)) * BTREE_PAGE_SIZE

	if err := extendMmap(db, size); err != nil {
		return err
	}

	offset := int64(db.page.flushed * BTREE_PAGE_SIZE)

	if _, err := unix.Pwritev(db.fd, db.page.temp, offset); err != nil {
		return err
	}

	// arunca data din memoria principala
	db.page.flushed += uint64(len(db.page.temp))
	db.page.temp = db.page.temp[:0]

	return nil
}
