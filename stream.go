package wrpc

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/duomi520/utils"
)

// Stream 流
type Stream struct {
	ctx           context.Context
	id            int64
	serviceMethod string
	marshal       func(any, io.Writer) error
	unmarshal     func([]byte, any) error
	send          WriterFunc
	payload       chan []byte
	closeOnce     sync.Once
}

func (s *Stream) Send(data any) error {
	var f Frame
	f.Status = utils.StatusStream16
	f.Seq = s.id
	f.ServiceMethod = s.serviceMethod
	f.Payload = data
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	err := f.MarshalBinary(s.marshal, buf)
	if err != nil {
		return fmt.Errorf("Stream.Send：marshal fail %s", err.Error())
	}
	err = s.send(buf.bytes())
	return err
}

// Recv 非顺序接受数据
func (s *Stream) Recv() (any, error) {
	var data []byte
	select {
	case data = <-s.payload:
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
	var v any
	err := s.unmarshal(data, &v)
	return v, err
}

// release 释放
func (s *Stream) release() {
	s.closeOnce.Do(func() {
		s.send(MarshalBinaryFrameCtxCancelFunc(s.id))
		close(s.payload)
	})
}
