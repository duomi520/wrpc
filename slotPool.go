package wrpc

import (
	"sync"
	"sync/atomic"
)

//SlotSize 槽大小
var SlotSize = 4 * 1024

var slotPool = sync.Pool{
	New: func() interface{} {
		var s slot
		s.buf = make([]byte, SlotSize)
		return &s
	},
}

//slot 槽
type slot struct {
	buf      []byte
	size     uint64
	usedSize uint64
}

func getSlot() *slot {
	return slotPool.Get().(*slot)
}

func (s *slot) setSize(n uint64) {
	atomic.StoreUint64(&s.size, n)
}

func (s *slot) used(n uint64) {
	new := atomic.AddUint64(&s.usedSize, n)
	if new > s.size {
		panic("slot.buf 使用字节累计超原始字节")
	}
	if new == s.size {
		s.release()
	}
}
func (s *slot) release() {
	atomic.StoreUint64(&s.size, 0)
	atomic.StoreUint64(&s.usedSize, 0)
	slotPool.Put(s)
}
