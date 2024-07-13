package wrpc

import (
	"context"
	"io"
	"log"
	"sync"

	"github.com/duomi520/utils"
)

// Stream 流
type Stream struct {
	ctx context.Context
	Conn
	serviceMethod string
	encoder       func(any, io.Writer) error
	unmarshal     func([]byte, any) error
	payload       chan []byte
	closeOnce     sync.Once
}

func (s *Stream) Send(data any) error {
	f := NewFrame(utils.StatusStream16, s.id, s.serviceMethod)
	return FrameEncode(f, utils.MetaDict[string]{}, data, s.w, s.encoder)
}

// Recv 非顺序接受数据
func (s *Stream) Recv() (any, error) {
	select {
	case data := <-s.payload:
		var v any
		err := s.unmarshal(data, &v)
		return v, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

// release 释放
func (s *Stream) release() {
	s.closeOnce.Do(func() {
		_, err := s.w.Write(FrameCtxCancelFunc(s.id))
		if err != nil {
			log.Println("release:" + err.Error())
		}
		close(s.payload)
	})
}
