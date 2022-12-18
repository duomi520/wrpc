package wrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/duomi520/utils"
)

//Client 连接
type Client struct {
	Options
	callMap sync.Map
	connect
	warpSend WriterFunc
	stopSign *int32
}

//NewTCPClient 新建
func NewTCPClient(url string, o *Options) (*Client, error) {
	c := &Client{
		Options: *o,
	}
	var err error
	c.Id, err = o.snowFlakeID.NextID()
	if err != nil {
		return nil, err
	}
	t, err := TCPDial(url, o.ProtocolMagicNumber, o.Hijacker, c.clientHandler, o.Logger)
	if err != nil {
		return nil, err
	}
	c.send = t.Send
	c.warpSend = func(b []byte) error {
		for v := range o.OutletHook {
			if b, err = o.OutletHook[v](b, t.Send); err != nil {
				c.Logger.Warn(err.Error())
				return nil
			}
		}
		return t.Send(b)
	}
	c.stopSign = t.stopSign
	return c, nil
}

//Close 关闭
func (c *Client) Close() {
	atomic.StoreInt32(c.stopSign, 1)
	c.send(frameGoaway)
}

//Send 发送
func (c *Client) Send(b []byte) error {
	return c.warpSend(b)
}

func (c *Client) sendFrame(ctx context.Context, status uint16, seq int64, serviceMethod string, args any) error {
	f := Frame{Status: status, Seq: seq, ServiceMethod: serviceMethod, Payload: args}
	if v := ctx.Value(metadataKey); v != nil {
		f.Metadata = v.(*utils.MetaDict)
	}
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	var err error
	if err = f.MarshalBinary(c.Marshal, buf); err != nil {
		return err
	}
	if err = c.AllowRequest(); err != nil {
		return err
	}
	if err = c.warpSend(buf.bytes()); err != nil {
		c.MarkFailed()
	} else {
		c.MarkSuccess()
	}
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
		f := make([]byte, 18)
		copy(f, frameCtxCancelFunc)
		utils.CopyInteger64(f[6:], id)
		c.send(f)
		return ctx.Err()
	case <-rc.Done:
		err = rc.Error
		//TODO 有泄露风险
		close(rc.Done)
		return err
	}
}

//TODO Notify模式

/*
//Go Go异步的调用函数。 待定
func (c *Client) Go(ctx context.Context, serviceMethod string, args, reply any, done chan struct{}) error {
	//TODO　ctx.Done 起作用
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
}*/

//NewStream
func (c *Client) NewStream(ctx context.Context, serviceMethod string) (*Stream, error) {
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return nil, fmt.Errorf("Client.NewStream: snowFlake id fail %s", err.Error())
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
	if v := GetMetadata(ctx); v != nil {
		f.Metadata = v
	}
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	err = f.MarshalBinary(s.marshal, buf)
	if err != nil {
		return nil, fmt.Errorf("Client.NewStream: 编码失败 %s", err.Error())
	}
	err = c.send(buf.bytes())
	if err != nil {
		return nil, fmt.Errorf("Client.NewStream: %s", err.Error())
	}
	go func() {
		<-ctx.Done()
		c.CloseStream(s)
	}()
	return s, nil
}

func (c *Client) CloseStream(s *Stream) {
	s.release()
	c.callMap.Delete(s.id)
}

//Subscribe 订阅主题
func (c *Client) Subscribe(topic string, handler WriterFunc) error {
	c.callMap.Store(topic, handler)
	return c.sendFrame(context.TODO(), utils.StatusSubscribe16, c.Id, topic, nil)
}

//Unsubscribe 退订主题
func (c *Client) Unsubscribe(topic string) error {
	c.callMap.Delete(topic)
	return c.sendFrame(context.TODO(), utils.StatusUnsubscribe16, c.Id, topic, nil)
}

func (c *Client) clientHandler(recv []byte, send WriterFunc, callback func()) error {
	var f Frame
	var n int
	var err error
	//入口拦截
	for v := range c.IntletHook {
		if recv, err = c.IntletHook[v](recv, send); err != nil {
			c.Logger.Warn(err.Error())
			return nil
		}
	}
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
			handler := v.(WriterFunc)
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
			return fmt.Errorf("Client.clientHandler：broadcast no found f.ServiceMethod with %s", f.ServiceMethod)
		}
	case utils.StatusStream16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			buf := make([]byte, len(recv[n:]))
			copy(buf, recv[n:])
			v.(*Stream).payload <- buf
		} else {
			return fmt.Errorf("Client.clientHandler：stream no found f.Seq with %d", f.Seq)
		}
	}
	return nil
}

// https://www.jianshu.com/p/0b7cf0bdbe92
// https://www.modb.pro/db/383217
