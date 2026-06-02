package internal

const (
	TABLE_SIZE = 128
	CACHE_CAP  = 100
)

type Bucket struct {
	head *BucketNode
	tail *BucketNode
}

type BucketNode struct {
	pagePtr    uint64
	lruPointer *LRUNode
	next       *BucketNode
}

type LRUNode struct {
	pagePtr  uint64
	pageData []byte
	next     *LRUNode
	prev     *LRUNode
}

type LRUList struct {
	head *LRUNode
	tail *LRUNode
}

type LRUCache struct {
	capacity int
	size     int
	// table reprezinta hashtable-ul
	table [TABLE_SIZE]*Bucket
	list  *LRUList
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

// Functie pentru transformarea unei chei
// intr-o valoare hash
// cheia este de tip uint64 deoarece reprezinta un pointer
// catre locatia fizica pe disc a nodului
func hashKey(key uint64) uint32 {
	hash := key * 2654435761
	return uint32(hash) & (TABLE_SIZE - 1)
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
	cache := &LRUCache{
		capacity: CACHE_CAP,
		size:     0,
		list:     initLRUList(),
	}

	for i := range cache.table {
		cache.table[i] = initBucketSentinel()
	}

	return cache
}

// Get verifica daca o cheie este stocata in hashtable
// si returneaza valoarea ei, respectiv un slice gol
// totodata in cazul de Cache Hit actualizeaza
// lista LRU
func (cache *LRUCache) Get(key uint64) []byte {
	hash := hashKey(key)
	// pornire dupa nodul santinela
	curr := cache.table[hash].head.next
	for curr != nil {
		if curr.pagePtr == key {
			break
		}
		curr = curr.next
	}
	if curr == nil {
		return nil
	}

	hitNode := curr.lruPointer
	head := cache.list.head
	hitNode.prev.next = hitNode.next
	hitNode.next.prev = hitNode.prev
	hitNode.next = head.next
	head.next.prev = hitNode
	hitNode.prev = head
	head.next = hitNode

	return hitNode.pageData
}
