package wrpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/duomi520/utils"
	"go/token"
	"reflect"
	"sync"
)

type connect struct {
	Id int64
	//发送
	send func([]byte) error
}

func (c connect) equal(obj any) bool {
	return c.Id == obj.(connect).Id
}

//MethodInfo 方法
type MethodInfo struct {
	method    reflect.Method
	val       reflect.Value
	argsType  reflect.Type
	replyType reflect.Type
	stream    *Stream
}

//Service 服务端应答
type Service struct {
	Options
	tcpServer *TCPServer
	methodMap map[string]MethodInfo
	topicMap  sync.Map
}

//NewService 新
func NewService(o *Options) *Service {
	return &Service{
		Options:   *o,
		methodMap: make(map[string]MethodInfo, 256),
	}
}

//TCPServer tcp服务
func (sh *Service) TCPServer(ctx context.Context, port string) error {
	if len(port) < 1 {
		return errors.New("TCPServer: 端口号为空")

	}
	sh.tcpServer = NewTCPServer(ctx, port, sh.serveRequest, sh.Logger)
	if sh.tcpServer == nil {
		return errors.New("TCPServer: 启动TCP服务失败")
	}
	sh.ProtocolMagicNumber = sh.Options.ProtocolMagicNumber
	go sh.tcpServer.Run()
	return nil
}

//RegisterTopic 注册主题
func (sh *Service) RegisterTopic(name string) *Topic {
	t := Topic{
		Service:      sh,
		Name:         name,
		audienceList: utils.NewLockList(),
	}
	sh.topicMap.Store(name, &t)
	return &t
}

//RemoveTopic 移除主题
func (sh *Service) RemoveTopic(name string) {
	v, ok := sh.topicMap.Load(name)
	if ok {
		v.(*Topic).audienceList = nil
		sh.topicMap.Delete(name)
	}
}

//RegisterRPC 函数必须是导出的，即首字母为大写
//函数长时间阻塞会影响后面的传输。
func (sh *Service) RegisterRPC(target string, rcvr any) error {
	t := reflect.TypeOf(rcvr)
	val := reflect.ValueOf(rcvr)
	for i := 0; i < t.NumMethod(); i++ {
		var m MethodInfo
		m.method = t.Method(i)
		rType := m.method.Type
		name := m.method.Name
		//如果是私有方法，PkgPath显示包名
		if m.method.PkgPath != "" {
			continue
		}
		// 判断是否是三个参数：第一个是结构本身，第二个是上下文，第三个是参数
		if rType.NumIn() != 3 {
			return fmt.Errorf("RegisterRPC: method %s has wrong number of ins: %d", name, rType.NumIn())
		}
		m.val = val
		m.argsType = rType.In(2)
		//args必须是可暴露的
		if !isExportedOrBuiltinType(m.argsType) {
			return fmt.Errorf("RegisterRPC: argument type of method %s is not exported: %q", name, m.argsType)
		}
		// 必须有两个或一个返回值
		if rType.NumOut() != 2 && rType.NumOut() != 1 {
			return fmt.Errorf("RegisterRPC: method %s has %d output parameters; needs exactly two or one", name, rType.NumOut())
		}
		if rType.NumOut() == 2 {
			m.replyType = rType.Out(0)
			// reply必须是可暴露的
			if !isExportedOrBuiltinType(m.replyType) {
				return fmt.Errorf("RegisterRPC: reply type of method %s is not exported: %q", name, m.replyType)
			}
			if returnType := rType.Out(1); returnType != reflect.TypeOf((*error)(nil)).Elem() {
				return fmt.Errorf("RegisterRPC: return type of method %s is %q, must be error", name, returnType)
			}
		}
		if rType.NumOut() == 1 {
			if returnType := rType.Out(0); returnType != reflect.TypeOf((*error)(nil)).Elem() {
				return fmt.Errorf("RegisterRPC: return type of method %s is %q, must be error", name, returnType)
			}
		}
		key := target + "." + name
		sh.methodMap[key] = m
	}
	return nil
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}

func (sh *Service) functionCall(key string, meta map[any]any, payload []byte) (any, error) {
	m, ok := sh.methodMap[key]
	if !ok {
		return nil, fmt.Errorf("functionCall: can't find service method %s", key)
	}
	args := reflect.New(m.argsType)
	if err := sh.Unmarshal(payload, args.Interface()); err != nil {
		return nil, err
	}
	params := make([]reflect.Value, 3)
	params[0] = m.val
	ctx := context.Background()
	for k, v := range meta {
		ctx = context.WithValue(ctx, k, v)
	}
	params[1] = reflect.ValueOf(ctx)
	params[2] = args.Elem()
	function := m.method.Func
	returnValues := function.Call(params)
	reply := reflect.New(m.replyType)
	_ = reply
	reply = returnValues[0]
	if err := returnValues[1].Interface(); err != nil {
		return nil, err.(error)
	}
	return reply.Interface(), nil
}
func (sh *Service) functionStream(id int64, key string, meta map[any]any, payload []byte, send func([]byte) error) error {
	m, ok := sh.methodMap[key]
	if !ok {
		return fmt.Errorf("functionStream: can't find service method %s", key)
	}
	if m.stream == nil {
		m.stream = &Stream{ctx: context.Background(), id: id, serviceMethod: key, send: send, marshal: sh.Marshal, unmarshal: sh.Unmarshal}
		m.stream.payload = make(chan []byte, 16)
		sh.methodMap[key] = m
		ctx := context.Background()
		for k, v := range meta {
			ctx = context.WithValue(ctx, k, v)
		}
		go func() {
			params := make([]reflect.Value, 3)
			params[0] = m.val
			params[1] = reflect.ValueOf(ctx)
			params[2] = reflect.ValueOf(m.stream)
			function := m.method.Func
			returnValues := function.Call(params)
			if err := returnValues[0].Interface(); err != nil {
				sh.Logger.Error("functionStream: ", err.(error).Error())
			}
			m.stream.free()
			delete(sh.methodMap, key)
		}()
		return nil
	}
	buf := make([]byte, len(payload))
	copy(buf, payload)
	m.stream.payload <- buf
	return nil
}
func (sh *Service) sendError(f Frame, send func([]byte) error, err error) error {
	f.Status = utils.StatusError16
	f.Payload = err.Error()
	buf, err := f.MarshalBinary(sh.Marshal, makeBytes)
	if err != nil {
		return fmt.Errorf("sendError: marshal fail %s: %w", err.Error(), err)
	}
	return send(buf)
}

//serveRequest h
func (sh *Service) serveRequest(send func([]byte) error, recv []byte) error {
	var f Frame
	var n int
	var err error
	if n, err = f.UnmarshalHeader(recv); err != nil {
		return err
	}
	switch f.Status {
	case utils.StatusSubscribe16:
		v, ok := sh.topicMap.Load(f.ServiceMethod)
		if ok {
			v.(*Topic).add(f.Seq, send)
		}
		return nil
	case utils.StatusUnsubscribe16:
		v, ok := sh.topicMap.Load(f.ServiceMethod)
		if ok {
			v.(*Topic).remove(f.Seq)
		}
		return nil
	case utils.StatusStream16:
		return sh.functionStream(f.Seq, f.ServiceMethod, f.Metadata, recv[n:], send)
	default:
		f.Payload, err = sh.functionCall(f.ServiceMethod, f.Metadata, recv[n:])
		if err != nil {
			return sh.sendError(f, send, err)
		}
	}
	f.Status = utils.StatusResponse16
	buf, err := f.MarshalBinary(sh.Marshal, makeBytes)
	if err == nil {
		err = send(buf)
	}
	return err
}

// https://github.com/golang/go/blob/master/src/net/rpc/server.go?name=release#227
// https://blog.csdn.net/yangyangye/article/details/52850121
// https://zhuanlan.zhihu.com/p/139384493
// https://github.com/smallnest/rpcx
