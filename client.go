package wrpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/duomi520/utils"
	"sync"
)

//Client 连接
type Client struct {
	Options
	callMap sync.Map
	connect
}

//NewTCPClient 新
func NewTCPClient(ctx context.Context, url string, o *Options) (*Client, error) {
	c := &Client{
		Options: *o,
	}
	var err error
	c.Id, err = o.snowFlakeID.NextID()
	if err != nil {
		return nil, err
	}
	t, err := TCPDial(ctx, url, o.ProtocolMagicNumber, c.clientHandler, o.Logger)
	if err != nil {
		return nil, err
	}
	c.send = t.Send
	return c, nil
}

func (c *Client) sendFrame(ctx context.Context, status uint16, seq int64, serviceMethod string, args any) error {
	f := Frame{Status: status, Seq: seq, ServiceMethod: serviceMethod, Payload: args}
	if v := ctx.Value(ContextKey); v != nil {
		f.Metadata = v.(map[any]any)
	}
	buf, err := f.MarshalBinary(c.Marshal, makeBytes)
	if err != nil {
		return err
	}
	err = c.send(buf)
	return err
}

type rpcResponse struct {
	id     int64
	client *Client
	reply  any
	Done   chan struct{}
	Error  error
}

var rpcResponsePool sync.Pool

//rpcResponseGet 从池里取一个
func rpcResponseGet() *rpcResponse {
	v := rpcResponsePool.Get()
	if v != nil {
		v.(*rpcResponse).Done = make(chan struct{})
		return v.(*rpcResponse)
	}
	var r rpcResponse
	r.Done = make(chan struct{})
	return &r
}

//rpcResponsePut 还一个到池里
func rpcResponsePut(r *rpcResponse) {
	r.client = nil
	r.reply = nil
	close(r.Done)
	rpcResponsePool.Put(r)
}

//Call 调用指定的服务，方法，等待调用返回，将结果写入reply，然后返回执行的错误状态
//request and response/请求-响应
func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return err
	}
	rc := rpcResponseGet()
	rc.id = id
	rc.client = c
	rc.reply = reply
	c.callMap.Store(id, rc)
	defer rpcResponsePut(rc)
	err = c.sendFrame(ctx, utils.StatusRequest16, id, serviceMethod, args)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rc.Done:
		err = rc.Error
		return err
	}
}

//Go Go异步的调用函数。
func (c *Client) Go(ctx context.Context, serviceMethod string, args, reply any, done chan struct{}) error {
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return err
	}
	if done == nil {
		return c.sendFrame(ctx, utils.StatusRequest16, id, serviceMethod, args)
	}
	var r rpcResponse
	r.Done = done
	r.id = id
	r.reply = reply
	r.client = c
	c.callMap.Store(id, &r)
	return c.sendFrame(ctx, utils.StatusRequest16, id, serviceMethod, args)
}

//NewStream
func (c *Client) NewStream(ctx context.Context, serviceMethod string) (*Stream, error) {
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return nil, fmt.Errorf("NewStream: snowFlake id fail %s", err.Error())
	}
	s := &Stream{ctx: ctx, id: id, serviceMethod: serviceMethod, marshal: c.Marshal, unmarshal: c.Unmarshal, send: c.send}
	s.payload = make(chan []byte, 16)
	c.callMap.Store(id, s)
	//首次发送metadata
	f := Frame{
		Status:        utils.StatusStream16,
		Seq:           id,
		ServiceMethod: serviceMethod,
		Payload:       nil,
	}
	if v := ctx.Value(ContextKey); v != nil {
		f.Metadata = v.(map[any]any)
	}
	buf, err := f.MarshalBinary(s.marshal, makeBytes)
	if err != nil {
		return nil, fmt.Errorf("NewStream: marshal fail %s", err.Error())
	}
	err = c.send(buf)
	if err != nil {
		return nil, fmt.Errorf("NewStream: %s", err.Error())
	}
	return s, nil
}

func (c *Client) CloseStream(s *Stream) {
	s.free()
	//TODO 通知服务器stream关闭
	c.callMap.Delete(s.id)
}

//Subscribe 订阅主题
func (c *Client) Subscribe(topic string, handler func([]byte) error) error {
	c.callMap.Store(topic, handler)
	return c.sendFrame(context.TODO(), utils.StatusSubscribe16, c.Id, topic, nil)
}

//Unsubscribe 退订主题
func (c *Client) Unsubscribe(topic string) error {
	c.callMap.Delete(topic)
	return c.sendFrame(context.TODO(), utils.StatusUnsubscribe16, c.Id, topic, nil)
}

func (c *Client) clientHandler(send func([]byte) error, recv []byte) error {
	var f Frame
	var n int
	var err error
	if n, err = f.UnmarshalHeader(recv); err != nil {
		return err
	}
	switch f.Status {
	case utils.StatusResponse16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			rc := v.(*rpcResponse)
			err = c.Unmarshal(recv[n:], rc.reply)
			rc.Error = err
			rc.Done <- struct{}{}
			c.callMap.Delete(f.Seq)
		}
	case utils.StatusError16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			rc := v.(*rpcResponse)
			var msg string
			err = c.Unmarshal(recv[n:], &msg)
			if err != nil {
				rc.Error = err
			} else {
				rc.Error = errors.New(msg)
			}
			rc.Done <- struct{}{}
			c.callMap.Delete(f.Seq)
		}
	case utils.StatusBroadcast16:
		v, ok := c.callMap.Load(f.ServiceMethod)
		if ok {
			handler := v.(func([]byte) error)
			var data []byte
			err = c.Unmarshal(recv[n:], &data)
			if err != nil {
				return err
			}
			err = handler(data)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("broadcast no found f.ServiceMethod with %s", f.ServiceMethod)
		}
	case utils.StatusStream16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			buf := make([]byte, len(recv[n:]))
			copy(buf, recv[n:])
			v.(*Stream).payload <- buf
		} else {
			return fmt.Errorf("stream no found f.Seq with %d", f.Seq)
		}
	}
	return nil
}
