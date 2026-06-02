package internal

const TABLE_SIZE = 128

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
func InitializeTable() *HashTable {
	ht := &HashTable{}
	for i := range ht.table {
		ht.table[i] = initBucketSentinel()
	}

	return ht
}
