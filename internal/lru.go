package internal

const (
	TABLE_SIZE  = 128
	CACHE_CAP   = 100
	INSERT_HASH = 1
	UPDATE_HASH = 2
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

// moveNodeHead muta nodul primit ca parametru
// daca acesta a fost un nod deja existent, il scoate de
// la locul lui si reface legaturile
func moveNodeHead(cache *LRUCache, hitNode *LRUNode, mode int) {
	head := cache.list.head
	if mode == UPDATE_HASH {
		hitNode.prev.next = hitNode.next
		hitNode.next.prev = hitNode.prev
	}

	hitNode.next = head.next
	head.next.prev = hitNode
	hitNode.prev = head
	head.next = hitNode
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
	moveNodeHead(cache, hitNode, UPDATE_HASH)
	return hitNode.pageData
}

// insertUpdateTable insereaza sau actualizeaza o pagina in hashtable
// Returneaza tipul (insert/update) si nodul inserat/actualizat
func insertUpdateTable(cache *LRUCache, key uint64, data []byte) (int, *LRUNode) {
	hash := hashKey(key)
	curr := cache.table[hash].head.next

	for curr != nil {
		if curr.pagePtr == key {
			hitNode := curr.lruPointer
			hitNode.pageData = data
			return UPDATE_HASH, hitNode
		}
		curr = curr.next
	}

	lruNode := &LRUNode{
		pagePtr:  key,
		pageData: data,
	}
	buckNode := &BucketNode{
		pagePtr:    key,
		lruPointer: lruNode,
	}

	cache.size++
	cache.table[hash].tail.next = buckNode
	cache.table[hash].tail = buckNode

	return INSERT_HASH, lruNode
}

// removeNodeListTable sterge un nod din lista de cache si din hashtable
func removeNodeListTable(cache *LRUCache) {
	key := cache.list.tail.prev.pagePtr
	cache.size--
	newLast := cache.list.tail.prev.prev
	newLast.next = cache.list.tail
	cache.list.tail.prev = newLast

	hash := hashKey(key)
	curr := cache.table[hash].head
	tail := cache.table[hash].tail
	for curr.next.pagePtr != key {
		curr = curr.next
	}

	deleted := curr.next
	if deleted == tail {
		tail = curr
	}

	curr.next = deleted.next
}

// Put adauga o pagina in hastable si in lista de caching
// in cazul existentei, actualizeaza valoarea stocata
// Daca capacitatea listei LRU depaseste 100, elimina
// ultima pagina, aceia find si cea mai putin utilizata
func (cache *LRUCache) Put(key uint64, data []byte) {
	mode, node := insertUpdateTable(cache, key, data)
	moveNodeHead(cache, node, mode)
	if cache.size > cache.capacity {
		removeNodeListTable(cache)
	}
}
