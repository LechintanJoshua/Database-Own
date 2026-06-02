package internal

const (
	TABLE_SIZE = 128
	CACHE_CAP  = 100
)

type HashTable struct {
	table [TABLE_SIZE]*Bucket
}

type Bucket struct {
	head *BucketNode
	tail *BucketNode
}

type BucketNode struct {
	key        []byte
	lruPointer *LRUNode
	next       *BucketNode
}

type LRUNode struct {
	key    []byte
	values []byte
	next   *LRUNode
	prev   *LRUNode
}

type LRUList struct {
	head *LRUNode
	tail *LRUNode
}

type LRUCache struct {
	capacity int
	size     int
	table    *HashTable
	list     *LRUList
}

// initBucketSentinel este folosita pentru a crea un nod santinela
// in lista inlantuita din bucket
func initBucketSentinel() *Bucket {
	buckNode := &BucketNode{}
	return &Bucket{
		head: buckNode,
		tail: buckNode,
	}
}

// InitializeTable initializeaza fiecare valoare din table cu
// o lista inlantuita ce contine un singur nod gol santinela
func initializeTable() *HashTable {
	ht := &HashTable{}
	for i := range ht.table {
		ht.table[i] = initBucketSentinel()
	}

	return ht
}

// Functie pentru transformarea unei chei intr-un
// hash folosind algoritmul FNV-1a pentur 32bits
func hashKey(key []byte) uint32 {
	var offset uint32 = 2166136261
	var prime uint32 = 16777619

	for _, b := range key {
		offset ^= uint32(b)
		offset *= prime
	}

	return offset & (TABLE_SIZE - 1)
}

// initLRUList initializeaza lista dubla inlantuita pentru
// mentinearea nodurilor folosite in cache
// lista este initializata cu doua noduri santinela goale
// tail si head pentru a evita edge-case-urile
func initLRUList() *LRUList {
	headNode := &LRUNode{}
	tailNode := &LRUNode{}
	headNode.next = tailNode
	tailNode.prev = headNode

	return &LRUList{
		head: headNode,
		tail: tailNode,
	}
}

// InitLRUCache initializeaza structura folosita pentrru
// un caching de tip Least Recently Used
// capacitatea este hardcodata la 100
// nodurile santinela vor fi ignorate
func InitLRUCache() *LRUCache {
	return &LRUCache{
		capacity: CACHE_CAP,
		size:     0,
		table:    initializeTable(),
		list:     initLRUList(),
	}
}
