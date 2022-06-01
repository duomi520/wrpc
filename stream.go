package wrpc

import (
	"context"
	"fmt"
	"github.com/duomi520/utils"
	"sync"
)

//Stream 流
type Stream struct {
	ctx           context.Context
	id            int64
	serviceMethod string
	marshal       func(any) ([]byte, error)
	unmarshal     func([]byte, any) error
	send          func([]byte) error
	payload       chan []byte
	closeOnce     sync.Once
}

func (s *Stream) Send(data any) error {
	var f Frame
	f.Status = utils.StatusStream16
	f.Seq = s.id
	f.ServiceMethod = s.serviceMethod
	f.Payload = data
	buf, err := f.MarshalBinary(s.marshal, makeBytes)
	if err != nil {
		return fmt.Errorf("marshal fail %s", err.Error())
	}
	err = s.send(buf)
	return err
}

func (s *Stream) Recv() (any, error) {
	data := <-s.payload
	var v any
	err := s.unmarshal(data, &v)
	return v, err
}

//Free 释放

func (s *Stream) free() {
	s.closeOnce.Do(func() {
		close(s.payload)
	})
}
