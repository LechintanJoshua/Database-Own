package internal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"syscall"

	"golang.org/x/sys/unix"
)

const DB_SIG = "BuildYourOwnDBLJ"

type KV struct {
	Path string
	fd   int
	tree BTree
	free FreeList

	mmap struct {
		total  int
		chunks [][]byte
	}

	page struct {
		flushed uint64
		nappend uint64
		updates map[uint64][]byte
	}

	failed bool
}

// Open deschide fisierul din adresa Path
// si verifica daca aceasta a avut succes
func (db *KV) Open() error {
	db.tree.get = db.pageRead
	db.tree.new = db.pageAlloc
	db.tree.del = db.free.PushTail

	db.free.get = db.pageRead
	db.free.new = db.pageAppend
	db.free.set = db.pageWrite

	db.page.updates = make(map[uint64][]byte)

	fd, err := createFileSync(db.Path)

	if err != nil {
		return err
	}

	db.fd = fd

	size, err := os.Stat(db.Path)

	if err != nil {
		return err
	}

	sz := size.Size()

	if sz > 0 {
		if err := extendMmap(db, int(sz)); err != nil {
			return err
		}
	}

	if err = readRoot(db, size.Size()); err != nil {
		return err
	}

	return nil
}

// Get apeleaza metoda interna a tree si returneaza
// un tuplu (cheie, gasit)
func (db *KV) Get(key []byte) ([]byte, bool) {
	return db.tree.Get(key)
}

// Set apeleaza emtoda interna a tree si
// actualizeaza fisierul
func (db *KV) Set(key []byte, val []byte) (bool, error) {
	return db.Update(key, val, MODE_UPSERT)
}

// Update apeleaza metoda interna a tree si
// actualizeaza fisierul
func (db *KV) Update(key []byte, val []byte, mode int) (bool, error) {
	req := &UpdateReq{
		Key:  key,
		Val:  val,
		Mode: mode,
	}

	meta := saveMeta(db)
	db.tree.Update(req)
	err := updateOrRevert(db, meta)

	return req.Added, err
}

// Del apeleaza metoda interna a tree, actualizeaza fisierul
// si returneaza un tuplu (sters, erroare)
func (db *KV) Del(key []byte) (bool, error) {
	meta := saveMeta(db)
	deleted := db.tree.Delete(key)
	return deleted, updateOrRevert(db, meta)
}

// Close sterge paginile din memoria RAM
// si inchide descriptorul de fisiere
func (db *KV) Close() error {
	var err error

	for _, chunk := range db.mmap.chunks {
		e := syscall.Munmap(chunk)

		if e != nil {
			err = e
		}
	}

	merr := err
	err = syscall.Close(db.fd)

	if merr != nil {
		return fmt.Errorf("delete map: %w", merr)
	}

	if err != nil {
		return fmt.Errorf("close fd: %w", err)
	}

	return nil
}

// updateFile actualizeaza fisierul si verifica erorile
// scrie nodurile in pagini, sincronizeaza, actualizeaza
// radacina si sincronizeaza din nou sa fie persistent
// (two-phase-update)
func updateFile(db *KV) error {
	if err := writePages(db); err != nil {
		return err
	}

	if err := syscall.Fsync(db.fd); err != nil {
		return err
	}

	if err := updateRoot(db); err != nil {
		return err
	}

	if err := syscall.Fsync(db.fd); err != nil {
		return err
	}

	db.free.SetMaxSeq()

	return nil
}

// createFileSync creaza un fisier nou,
// seteaza masca la -rw-r--r-- si sincronizeaza
// directorul in care se afla
func createFileSync(file string) (int, error) {
	flags := os.O_RDONLY | syscall.O_DIRECTORY
	dirfd, err := syscall.Open(path.Dir(file), flags, 0o644)

	if err != nil {
		return -1, fmt.Errorf("open director: %w", err)
	}

	defer syscall.Close(dirfd)

	flags = os.O_RDWR | os.O_CREATE
	fd, err := syscall.Openat(dirfd, path.Base(file), flags, 0o644)

	if err != nil {
		return -1, fmt.Errorf("open file: %w", err)
	}

	if err = syscall.Fsync(dirfd); err != nil {
		_ = syscall.Close(fd) // s-ar putea sa lase un fisier gol
		return -1, fmt.Errorf("fsync directory: %w", err)
	}

	return fd, nil
}

// pageRead verifica daca nodul a fost adus in memorie deja
// daca nu il citeste de pe disc
func (db *KV) pageRead(ptr uint64) []byte {
	if node, ok := db.page.updates[ptr]; ok {
		return node
	}

	return db.pageReadFile(ptr)
}

// pageReadFile citeste o pagina din memoria unui fisier
// si returneaza nodul asociat acelei pagini
func (db *KV) pageReadFile(ptr uint64) []byte {
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

	// 64 << 20 (64MB)
	alloc := max(db.mmap.total, 64<<20)

	for db.mmap.total+alloc < size {
		alloc *= 2
	}

	chunk, err := syscall.Mmap(
		db.fd, int64(db.mmap.total), alloc,
		syscall.PROT_READ, syscall.MAP_SHARED,
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
// returnam ptr pentru numarul nodului in pagina
func (db *KV) pageAppend(node []byte) uint64 {
	ptr := db.page.flushed + db.page.nappend
	db.page.updates[ptr] = node
	db.page.nappend++

	return ptr
}

// writePages scrie paginile cu noduri stocate temporar
// in fisierul de pe disc si verifica de erori
// actualizeaza numarul paginilor totale si reseteaza temp
func writePages(db *KV) error {
	size := (int(db.page.flushed) + int(db.page.nappend)) * BTREE_PAGE_SIZE

	if err := extendMmap(db, size); err != nil {
		return err
	}

	ptr := db.sortIdsAsc()

	for _, id := range ptr {
		node := db.page.updates[id]
		offset := int64(id * BTREE_PAGE_SIZE)

		if _, err := unix.Pwrite(db.fd, node, offset); err != nil {
			return err
		}
	}

	db.page.flushed += db.page.nappend
	db.page.nappend = 0
	clear(db.page.updates)

	return nil
}

// saveMeta salveaza meta datele bazei de date
// ii ofera semnatura, pointerul catre radacina,
// si numarul de pagini folosite precum si
// datele din FreeList
func saveMeta(db *KV) []byte {
	var data [64]byte
	copy(data[:16], []byte(DB_SIG))
	binary.LittleEndian.PutUint64(data[16:], db.tree.root)
	binary.LittleEndian.PutUint64(data[24:], db.page.flushed)
	binary.LittleEndian.PutUint64(data[32:], db.free.headPage)
	binary.LittleEndian.PutUint64(data[40:], db.free.headSeq)
	binary.LittleEndian.PutUint64(data[48:], db.free.tailPage)
	binary.LittleEndian.PutUint64(data[56:], db.free.tailSeq)

	return data[:]
}

// loadMeta citeste meta datele bazei din fisier
func loadMeta(db *KV, data []byte) {
	if string(data[:16]) != DB_SIG {
		panic("different signature")
	}

	db.tree.root = binary.LittleEndian.Uint64(data[16:])
	db.page.flushed = binary.LittleEndian.Uint64(data[24:])
	db.free.headPage = binary.LittleEndian.Uint64(data[32:])
	db.free.headSeq = binary.LittleEndian.Uint64(data[40:])
	db.free.tailPage = binary.LittleEndian.Uint64(data[48:])
	db.free.tailSeq = binary.LittleEndian.Uint64(data[56:])
}

// readRoot citeste radacina arborelui din datele
// meta ale fisierului si verifica daca acestea
// au fost corupte
func readRoot(db *KV, fileSize int64) error {
	if fileSize == 0 {
		db.page.flushed = 2
		db.free.headPage = 1
		db.free.tailPage = 1
		return nil
	}

	data := db.mmap.chunks[0]
	loadMeta(db, data)

	if db.tree.root >= db.page.flushed {
		err := errors.New("Corupted data, root >= pages")
		return err
	}

	totPages := db.page.flushed * BTREE_PAGE_SIZE

	if totPages > uint64(fileSize) {
		err := errors.New("Lost pages, more pages than writter")
		return err
	}

	return nil
}

// updateRoot scrie meta datele pe disc si verifica
// daca scrierea a avut succes
func updateRoot(db *KV) error {
	if _, err := syscall.Pwrite(db.fd, saveMeta(db), 0); err != nil {
		return fmt.Errorf("write meta page: %w", err)
	}

	return nil
}

// updateOrRevert verifica daca actualizarea a avut
// succes si repara in cazul in care nu
// verifica daca metadatele de pe disc sunt
// in stadiu necunoscut si le marcheaza asa pentru
// rollback
func updateOrRevert(db *KV, meta []byte) error {
	if db.failed {
		err := updateRoot(db)

		if err != nil {
			return fmt.Errorf("Update failed again: %w", err)
		}

		err = syscall.Fsync(db.fd)

		if err != nil {
			return fmt.Errorf("Fsync failed inside update recovery: %w", err)
		}

		db.failed = false
	}

	err := updateFile(db)

	if err != nil {
		db.failed = true
		loadMeta(db, meta)
		clear(db.page.updates)
		db.page.nappend = 0
	}

	return err
}

// pageAlloc aloca o pagina de memorie pe disc
// incearca sa recicleze o pagina din Free List
// daca nu reuseste da append
func (db *KV) pageAlloc(node []byte) uint64 {
	if ptr := db.free.PopHead(); ptr != 0 {
		db.page.updates[ptr] = node
		return ptr
	}

	return db.pageAppend(node)
}

// pageWrite aduce pagina unui nod din memorie
// verifica daca este pagina noua creata ce nu a fost scrisa
// sau o aduce din fisier
func (db *KV) pageWrite(ptr uint64) []byte {
	if node, ok := db.page.updates[ptr]; ok {
		return node
	}

	node := make([]byte, BTREE_PAGE_SIZE)
	copy(node, db.pageReadFile(ptr))
	db.page.updates[ptr] = node
	return node
}

// sortIdsAsc sorteaza id-urile paginilor
// care vor trebui actualizate crescator
func (db *KV) sortIdsAsc() []uint64 {
	ptr := make([]uint64, 0, len(db.page.updates))

	for id := range db.page.updates {
		ptr = append(ptr, id)
	}

	slices.Sort(ptr)

	return ptr
}
