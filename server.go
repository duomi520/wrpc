package wrpc

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"reflect"
	"strings"
	"sync"

	"github.com/duomi520/utils"
)

type connect struct {
	Id int64
	//发送
	send WriterFunc
}

func (c connect) equal(obj any) bool {
	return c.Id == obj.(connect).Id
}

//MethodInfo 方法
type MethodInfo struct {
	nonblock  bool
	method    reflect.Method
	val       reflect.Value
	argsType  reflect.Type
	replyType reflect.Type
	stream    *Stream
}

//Service 服务端应答
type Service struct {
	Options
	tcpServer      *TCPServer
	methodMap      map[string]*MethodInfo
	ctxExitFuncMap sync.Map
	topicMap       sync.Map
}

//NewService 新建
func NewService(o *Options) *Service {
	return &Service{
		Options:   *o,
		methodMap: make(map[string]*MethodInfo, 256),
	}
}

//TCPServer tcp服务
func (sh *Service) TCPServer(port string) error {
	if len(port) < 1 {
		return errors.New("Service.TCPServer: 端口号为空")

	}
	sh.tcpServer = NewTCPServer(port, sh.Hijacker, sh.serveRequest, sh.Logger)
	if sh.tcpServer == nil {
		return errors.New("Service.TCPServer: 启动TCP服务失败")
	}
	sh.ProtocolMagicNumber = sh.Options.ProtocolMagicNumber
	go sh.tcpServer.Run()
	return nil
}

//Stop
func (sh *Service) Stop() {
	sh.tcpServer.Stop()
}

//RegisterTopic 注册主题
func (sh *Service) RegisterTopic(name string) *Topic {
	t := Topic{
		Service:      sh,
		Name:         name,
		audienceList: utils.NewCopyOnWriteList(),
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
			return fmt.Errorf("Service.RegisterRPC: method %s has wrong number of ins: %d", name, rType.NumIn())
		}
		m.val = val
		m.argsType = rType.In(2)
		//args必须是可暴露的
		if !isExportedOrBuiltinType(m.argsType) {
			return fmt.Errorf("Service.RegisterRPC: argument type of method %s is not exported: %q", name, m.argsType)
		}
		// 必须有两个或一个返回值
		if rType.NumOut() != 2 && rType.NumOut() != 1 {
			return fmt.Errorf("Service.RegisterRPC: method %s has %d output parameters; needs exactly two or one", name, rType.NumOut())
		}
		if rType.NumOut() == 2 {
			m.replyType = rType.Out(0)
			// reply必须是可暴露的
			if !isExportedOrBuiltinType(m.replyType) {
				return fmt.Errorf("Service.RegisterRPC: reply type of method %s is not exported: %q", name, m.replyType)
			}
			if returnType := rType.Out(1); returnType != reflect.TypeOf((*error)(nil)).Elem() {
				return fmt.Errorf("Service.RegisterRPC: return type of method %s is %q, must be error", name, returnType)
			}
		}
		if rType.NumOut() == 1 {
			if returnType := rType.Out(0); returnType != reflect.TypeOf((*error)(nil)).Elem() {
				return fmt.Errorf("Service.RegisterRPC: return type of method %s is %q, must be error", name, returnType)
			}
		}
		key := target + "." + name
		sh.methodMap[key] = &m
	}
	return nil
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}
func (sh *Service) SetNonblock(key string) {
	m, ok := sh.methodMap[key]
	if ok {
		m.nonblock = true
	}
}
func (sh *Service) functionCall(id int64, m *MethodInfo, meta *utils.MetaDict, payload []byte) (any, error) {
	args := reflect.New(m.argsType)
	if err := sh.Unmarshal(payload, args.Interface()); err != nil {
		return nil, err
	}
	params := make([]reflect.Value, 3)
	params[0] = m.val
	ctx, cancel := context.WithCancel(context.Background())
	sh.ctxExitFuncMap.Store(id, cancel)
	defer sh.ctxExitFuncMap.Delete(id)
	if meta != nil {
		ctx = context.WithValue(ctx, metadataKey, meta)
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
func (sh *Service) functionStream(id int64, key string, meta *utils.MetaDict, payload []byte, send WriterFunc) error {
	m, ok := sh.methodMap[key]
	if !ok {
		return fmt.Errorf("Service.functionStream: can't find service method %s", key)
	}
	if m.stream == nil {
		ctx, cancel := context.WithCancel(context.Background())
		sh.ctxExitFuncMap.Store(id, cancel)
		m.stream = &Stream{ctx: ctx, id: id, serviceMethod: key, send: send, marshal: sh.Marshal, unmarshal: sh.Unmarshal}
		m.stream.payload = make(chan []byte, 16)
		sh.methodMap[key] = m
		if meta != nil {
			m.stream.ctx = MetadataContext(m.stream.ctx, meta)
		}
		go func() {
			defer func() {
				buf, r := formatRecover()
				if r != nil {
					sh.Logger.Error(fmt.Sprintf("Service.functionStream：异常拦截： %s \n%s", r, buf))
				}
			}()
			params := make([]reflect.Value, 3)
			params[0] = m.val
			params[1] = reflect.ValueOf(m.stream.ctx)
			params[2] = reflect.ValueOf(m.stream)
			function := m.method.Func
			returnValues := function.Call(params)
			if err := returnValues[0].Interface(); err != nil {
				if !strings.Contains(err.(error).Error(), "context canceled") {
					sh.Logger.Error("Service.functionStream: ", err.(error).Error())
				}
			}
			m.stream.release()
			delete(sh.methodMap, key)
			sh.ctxExitFuncMap.Delete(id)
		}()
		return nil
	}
	buf := make([]byte, len(payload))
	copy(buf, payload)
	m.stream.payload <- buf
	return nil
}
func (sh *Service) sendError(f Frame, send WriterFunc, err error) error {
	f.Status = utils.StatusError16
	f.Payload = err.Error()
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	err = f.MarshalBinary(sh.Marshal, buf)
	if err != nil {
		return fmt.Errorf("Service.sendError: 编码失败 %s: %w", err.Error(), err)
	}
	return send(buf.bytes())
}
func (sh *Service) sendResponse(f Frame, send WriterFunc) error {
	f.Status = utils.StatusResponse16
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	err := f.MarshalBinary(sh.Marshal, buf)
	if err == nil {
		err = send(buf.bytes())
	}
	return err
}

//serveRequest h
func (sh *Service) serveRequest(recv []byte, send WriterFunc, callback func()) error {
	var f Frame
	var n int
	var err error
	//入口拦截
	for v := range sh.IntletHook {
		if recv, err = sh.IntletHook[v](recv, send); err != nil {
			f.UnmarshalHeader(recv)
			sh.sendError(f, send, err)
			sh.Logger.Warn(err.Error())
			return nil
		}
	}
	//出口拦截
	warpSend := func(b []byte) error {
		for v := range sh.OutletHook {
			if b, err = sh.OutletHook[v](b, send); err != nil {
				sh.Logger.Warn(err.Error())
				return nil
			}
		}
		return send(b)
	}
	if n, err = f.UnmarshalHeader(recv); err != nil {
		callback()
		return err
	}
	switch f.Status {
	case utils.StatusSubscribe16:
		v, ok := sh.topicMap.Load(f.ServiceMethod)
		if ok {
			v.(*Topic).add(f.Seq, warpSend)
		}
		callback()
		return nil
	case utils.StatusUnsubscribe16:
		v, ok := sh.topicMap.Load(f.ServiceMethod)
		if ok {
			v.(*Topic).remove(f.Seq)
		}
		callback()
		return nil
	case utils.StatusStream16:
		callback()
		return sh.functionStream(f.Seq, f.ServiceMethod, f.Metadata, recv[n:], warpSend)
	case utils.StatusCtxCancelFunc16:
		cancel, ok := sh.ctxExitFuncMap.Load(f.Seq)
		if ok {
			cancel.(context.CancelFunc)()
			sh.ctxExitFuncMap.Delete(f.Seq)
		}
		callback()
		return nil
	default:
		m, ok := sh.methodMap[f.ServiceMethod]
		if !ok {
			return sh.sendError(f, send, fmt.Errorf("Service.serveRequest: can't find service method %s", f.ServiceMethod))
		}
		if m.nonblock {
			f.Payload, err = sh.functionCall(f.Seq, m, f.Metadata, recv[n:])
			if err != nil {
				return sh.sendError(f, send, err)
			}
			err = sh.sendResponse(f, warpSend)
			callback()
		} else {
			go func() {
				defer func() {
					callback()
					buf, r := formatRecover()
					if r != nil {
						sh.Logger.Error(fmt.Sprintf("Service.serveRequest %s \n%s", r, buf))
					}
				}()
				f.Payload, err = sh.functionCall(f.Seq, m, f.Metadata, recv[n:])
				if err != nil {
					sh.sendError(f, send, err)
					return
				}
				err = sh.sendResponse(f, warpSend)
				if err != nil {
					sh.Logger.Error(fmt.Sprintf("Service.serveRequest: 编码失败 %s", err.Error()))
				}
			}()
		}
	}
	return err
}

// https://github.com/golang/go/blob/master/src/net/rpc/server.go?name=release#227
// https://blog.csdn.net/yangyangye/article/details/52850121
// https://zhuanlan.zhihu.com/p/139384493
// https://github.com/smallnest/rpcx
