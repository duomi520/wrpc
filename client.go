package wrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/duomi520/utils"
)

// Client 连接
type Client struct {
	Options
	callMap sync.Map
	Conn
	stopSign *int32
	Close    func()
}

// NewTCPClient 新建
func NewTCPClient(url string, o *Options) (*Client, error) {
	c := &Client{
		Options: *o,
	}
	var err error
	c.id, err = o.snowFlakeID.NextID()
	if err != nil {
		return nil, err
	}
	t, err := TCPDial(url, o.ProtocolMagicNumber, c.clientHandler, o.Logger)
	if err != nil {
		return nil, err
	}
	c.w = t.conn
	c.stopSign = &t.stopFlag
	c.Close = t.close
	return c, nil
}

func (c *Client) sendFrame(ctx context.Context, status uint16, seq int64, serviceMethod string, args any) error {
	if err := c.AllowRequest(); err != nil {
		return err
	}
	f := NewFrame(status, seq, serviceMethod)
	meta := GetMeta(ctx)
	err := FrameEncode(f, meta, args, c.w, c.Encoder)
	if err != nil {
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

// Call 调用指定的服务，方法，等待调用返回，将结果写入reply，然后返回执行的错误状态
// request and response/请求-响应
func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	// 先判断是否本机有提供服务，有，先本机。
	if v, ok := c.methodMap[utils.Hash64FNV1A(serviceMethod)]; ok {
		r, err := directCall(ctx, v, args)
		//TODO 不合理，待优化
		if err == nil {
			buf, ex := c.Marshal(r)
			if ex == nil {
				return c.Unmarshal(buf, reply)
			}
			return ex
		}
		return err
	}
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return err
	}
	rc := AllocRPCResponse()
	defer FreeRPCResponse(rc)
	rc.id = id
	rc.client = c
	rc.reply = reply
	c.callMap.Store(id, rc)
	defer c.callMap.Delete(id)
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

//TODO Notify模式

/*
//Go Go异步的调用函数。 待定
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
}*/

// NewStream
func (c *Client) NewStream(ctx context.Context, serviceMethod string) (*Stream, error) {
	id, err := c.snowFlakeID.NextID()
	if err != nil {
		return nil, fmt.Errorf("Client.NewStream: snowFlake id fail %s", err.Error())
	}
	s := &Stream{ctx: ctx, serviceMethod: serviceMethod, encoder: c.Encoder, unmarshal: c.Unmarshal}
	s.id = id
	s.w = c.w
	s.payload = make(chan []byte, 16)
	c.callMap.Store(id, s)
	//首次发送metadata
	f := NewFrame(utils.StatusStream16, id, serviceMethod)
	meta := GetMeta(ctx)
	err = FrameEncode(f, meta, nil, s.w, s.encoder)
	if err != nil {
		return nil, fmt.Errorf("Client.NewStream: 编码失败 %s", err.Error())
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

// Subscribe 订阅主题
func (c *Client) Subscribe(topic string, handler func([]byte) error) error {
	c.callMap.Store(utils.Hash64FNV1A(topic), handler)
	return c.sendFrame(context.TODO(), utils.StatusSubscribe16, c.id, topic, nil)
}

// Unsubscribe 退订主题
func (c *Client) Unsubscribe(topic string) error {
	c.callMap.Delete(utils.Hash64FNV1A(topic))
	return c.sendFrame(context.TODO(), utils.StatusUnsubscribe16, c.id, topic, nil)
}

func (c *Client) clientHandler(recv []byte, w io.Writer) error {
	f, err := GetFrame(recv)
	if err != nil {
		return err
	}
	_, req, err := splitMetaPayload(recv)
	if err != nil {
		return err
	}
	switch f.Status {
	case utils.StatusResponse16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			rc := v.(*rpcResponse)
			rc.Error = c.Unmarshal(req, rc.reply)
			rc.Done <- struct{}{}
			c.callMap.Delete(f.Seq)
		}
	case utils.StatusError16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			rc := v.(*rpcResponse)
			var msg string
			err = c.Unmarshal(req, &msg)
			if err != nil {
				rc.Error = err
			} else {
				rc.Error = errors.New(msg)
			}
			rc.Done <- struct{}{}
			c.callMap.Delete(f.Seq)
		}
	case utils.StatusBroadcast16:
		v, ok := c.callMap.Load(f.Method)
		if ok {
			handler := v.(func([]byte) error)
			var data []byte
			err = c.Unmarshal(req, &data)
			if err != nil {
				return err
			}
			err = handler(data)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("Client.clientHandler: broadcast no found f.ServiceMethod")
		}
	case utils.StatusStream16:
		v, ok := c.callMap.Load(f.Seq)
		if ok {
			buf := make([]byte, len(req))
			copy(buf, req)
			v.(*Stream).payload <- buf
		} else {
			return fmt.Errorf("Client.clientHandler: stream no found f.Seq with %d", f.Seq)
		}
	}
	return nil
}

// https://www.jianshu.com/p/0b7cf0bdbe92
// https://www.modb.pro/db/383217
