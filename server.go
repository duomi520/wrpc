package wrpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/duomi520/utils"
	"go/token"
	"io"
	"reflect"
	"strings"
	"sync"
)

// Service 服务端应答
type Service struct {
	Options
	tcpServer      *TCPServer
	ctxExitFuncMap sync.Map
	topicMap       sync.Map
}

// NewService 新建
func NewService(o *Options) *Service {
	return &Service{
		Options: *o,
	}
}

// TCPServer tcp服务
func (sh *Service) TCPServer(port string) error {
	if len(port) < 1 {
		return errors.New("Service.TCPServer: 端口号为空")
	}
	hander := sh.serveRequest
	if sh.WarpHandler != nil {
		hander = sh.WarpHandler(hander)
	}
	sh.tcpServer = NewTCPServer(sh.snowFlakeID, port, hander, sh.Logger)
	if sh.tcpServer == nil {
		return errors.New("Service.TCPServer: 启动TCP服务失败")
	}
	sh.ProtocolMagicNumber = sh.Options.ProtocolMagicNumber
	go sh.tcpServer.Run()
	return nil
}

// Stop
func (sh *Service) Stop() {
	sh.tcpServer.Stop()
}

// RegisterTopic 注册主题
func (sh *Service) RegisterTopic(name string) *Topic {
	t := Topic{
		Service:      sh,
		Name:         name,
		audienceList: utils.NewCopyOnWriteList(),
	}
	sh.topicMap.Store(utils.Hash64FNV1A(name), &t)
	return &t
}

// RemoveTopic 移除主题
func (sh *Service) RemoveTopic(name string) {
	key := utils.Hash64FNV1A(name)
	v, ok := sh.topicMap.Load(key)
	if ok {
		v.(*Topic).audienceList = nil
		sh.topicMap.Delete(key)
	}
}

// RegisterRPC 函数必须是导出的，即首字母为大写
// 函数阻塞会打断网络io
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
		sh.methodMap[utils.Hash64FNV1A(key)] = &m
	}
	return nil
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}

func directCall(ctx context.Context, m *MethodInfo, args any) (any, error) {
	params := make([]reflect.Value, 3)
	params[0] = m.val
	params[1] = reflect.ValueOf(&RPCContext{Metadata: GetMeta(ctx)})
	params[2] = reflect.ValueOf(args)
	function := m.method.Func
	returnValues := function.Call(params)
	r := returnValues[0].Interface()
	if err := returnValues[1].Interface(); err != nil {
		return nil, err.(error)
	}
	return r, nil
}

func (sh *Service) functionCall(m *MethodInfo, meta utils.MetaDict[string], payload []byte) (any, error) {
	args := reflect.New(m.argsType)
	if err := sh.Unmarshal(payload, args.Interface()); err != nil {
		return nil, err
	}
	params := make([]reflect.Value, 3)
	params[0] = m.val
	ctx := &RPCContext{Metadata: meta}
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
func (sh *Service) functionStream(id int64, key uint64, meta utils.MetaDict[string], payload []byte, w io.Writer) error {
	m, ok := sh.methodMap[key]
	if !ok {
		return errors.New("Service.functionStream: can't find service method")
	}
	if m.stream == nil {
		ctx, cancel := context.WithCancel(context.Background())
		sh.ctxExitFuncMap.Store(id, cancel)
		m.stream = &Stream{ctx: ctx, encoder: sh.Encoder, unmarshal: sh.Unmarshal}
		m.stream.id = id
		m.stream.w = w
		m.stream.payload = make(chan []byte, 16)
		sh.methodMap[key] = m
		if meta.Len() != 0 {
			m.stream.ctx = SetMeta(m.stream.ctx, meta)
		}
		go func() {
			defer func() {
				buf, r := utils.FormatRecover()
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
					sh.Logger.Error("Service.functionStream: " + err.(error).Error())
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

func (sh *Service) sendError(f Frame, w io.Writer, err error) error {
	f.Status = utils.StatusError16
	e := FrameEncode(f, utils.MetaDict[string]{}, err.Error(), w, sh.Encoder)
	if e != nil {
		return fmt.Errorf("Service.sendError: 编码失败 %s: %w", err.Error(), err)
	}
	return nil
}

// serveRequest h
func (sh *Service) serveRequest(recv []byte, w io.Writer) error {
	f, err := GetFrame(recv)
	if err != nil {
		return err
	}
	switch f.Status {
	case utils.StatusSubscribe16:
		v, ok := sh.topicMap.Load(f.Method)
		if ok {
			v.(*Topic).add(f.Seq, w)
		}
	case utils.StatusUnsubscribe16:
		v, ok := sh.topicMap.Load(f.Method)
		if ok {
			v.(*Topic).remove(f.Seq)
		}
	case utils.StatusStream16:
		meta, req, err := splitMetaPayload(recv)
		if err != nil {
			sh.sendError(f, w, err)
			return err
		}
		return sh.functionStream(f.Seq, f.Method, meta, req, w)
	case utils.StatusCtxCancelFunc16:
		cancel, ok := sh.ctxExitFuncMap.Load(f.Seq)
		if ok {
			cancel.(context.CancelFunc)()
			sh.ctxExitFuncMap.Delete(f.Seq)
		}
	default:
		m, ok := sh.methodMap[f.Method]
		if !ok {
			return sh.sendError(f, w, errors.New("Service.serveRequest: can't find service"))
		}
		meta, req, err := splitMetaPayload(recv)
		if err != nil {
			return sh.sendError(f, w, err)
		}
		resp, err := sh.functionCall(m, meta, req)
		if err != nil {
			return sh.sendError(f, w, err)
		}
		f.Status = utils.StatusResponse16
		err = FrameEncode(f, utils.MetaDict[string]{}, resp, w, sh.Encoder)
		if err != nil {
			sh.Logger.Error(fmt.Sprintf("Service.serveRequest: 发送失败 %s", err.Error()))
		}
	}
	return nil
}

// https://github.com/golang/go/blob/master/src/net/rpc/server.go?name=release#227
// https://blog.csdn.net/yangyangye/article/details/52850121
// https://zhuanlan.zhihu.com/p/139384493
// https://github.com/smallnest/rpcx
