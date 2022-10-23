package wrpc

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

//SlotSize 槽大小
var SlotSize = 1024 * 4

var slotPool = sync.Pool{
	New: func() interface{} {
		var s slot
		s.buf = make([]byte, SlotSize)
		return &s
	},
}

//slot 槽
type slot struct {
	buf        []byte
	lenght     uint64
	usedLenght uint64
}

func getSlot() *slot {
	return slotPool.Get().(*slot)
}

func (s *slot) setLen(n uint64) {
	atomic.StoreUint64(&s.lenght, n)
}

//used 累计使用后释放
func (s *slot) used(n uint64) {
	new := atomic.AddUint64(&s.usedLenght, n)
	if new > s.lenght {
		panic(fmt.Sprintf("slot.buf 使用字节累计超原始字节 %d %d \n", int(new), int(s.lenght)))
	}
	if new == s.lenght {
		s.release()
	}
}

//release 释放
func (s *slot) release() {
	atomic.StoreUint64(&s.lenght, 0)
	atomic.StoreUint64(&s.usedLenght, 0)
	slotPool.Put(s)
}

//defaultBufferPoolLenght 长度
var defaultBufferPoolLenght = 512

//bufferPool 池
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &buffer{
			buf:   make([]byte, defaultBufferPoolLenght),
			valid: 0,
		}
	},
}

//buffer 缓存
type buffer struct {
	buf   []byte
	valid int
}

func (b *buffer) setValid(v int) {
	b.valid = v
}

func (b *buffer) cap() int {
	return len(b.buf)
}

//getbuf 取得底层[]byte
func (b *buffer) getbuf() []byte {
	return b.buf
}
func (b *buffer) reset() {
	b.valid = 0
}

//bytes 取得写入的[]byte
func (b *buffer) bytes() []byte {
	return b.buf[:b.valid]
}

//grow 扩大底层[]byte
func (b *buffer) grow(n int) {
	base := make([]byte, len(b.buf)+n)
	copy(base, b.buf)
	b.buf = base
}

//Write 实现write接口
func (b *buffer) Write(p []byte) (n int, err error) {
	if len(p) > (len(b.buf) - b.valid) {
		return 0, errors.New("超出buffer的底层[]byte的长度")
	}
	copy(b.buf[b.valid:], p)
	b.valid += len(p)
	return len(p), nil
}
