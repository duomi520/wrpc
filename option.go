package wrpc

import (
	"encoding/json"
	"io"
	"time"

	"github.com/duomi520/utils"
)

var SnowFlakeStartupTime int64 = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()

type WriterFunc func([]byte) error

func doNothing() {}

//Options 配置
type Options struct {
	ProtocolMagicNumber uint32
	snowFlakeID         *utils.SnowFlakeID
	//编码器
	Marshal func(any, io.Writer) error
	//解码器
	Unmarshal func([]byte, any) error
	//劫持者
	Hijacker func([]byte, WriterFunc) error
	//入口拦截器
	IntletHook []func([]byte, WriterFunc) ([]byte, error)
	//出口拦截器
	OutletHook []func([]byte, WriterFunc) ([]byte, error)
	//熔断器
	AllowRequest func() error
	MarkSuccess  func()
	MarkFailed   func()
	//平衡器
	//Balancer func([]int) int
	//Registry  IRegistry
	//日志
	Logger utils.ILogger
}

//Option 选项赋值
type Option func(*Options)

//NewOptions 创建并返回一个配置：接收Option函数类型的不定向参数列表
func NewOptions(opts ...Option) *Options {
	o := Options{}
	//设置默认值
	o.ProtocolMagicNumber = defaultProtocolMagicNumber
	o.snowFlakeID = utils.NewSnowFlakeID(0, SnowFlakeStartupTime)
	o.Logger, _ = utils.NewWLogger(utils.FatalLevel, "")
	o.Marshal = jsonMarshal
	o.Unmarshal = json.Unmarshal
	o.AllowRequest = func() error {
		return nil
	}
	o.MarkSuccess = doNothing
	o.MarkFailed = doNothing
	for _, v := range opts {
		v(&o)
	}
	return &o
}

func WithProtocolMagicNumber(pm uint32) Option {
	return func(o *Options) {
		o.ProtocolMagicNumber = pm
	}
}

//WithLogger 日志
func WithLogger(l utils.ILogger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

func jsonMarshal(o any, w io.Writer) error {
	return json.NewEncoder(w).Encode(o)
}

//WithCodec 编码
func WithCodec(m func(any, io.Writer) error, um func([]byte, any) error) Option {
	return func(o *Options) {
		if m != nil && um != nil {
			o.Marshal = m
			o.Unmarshal = um
		}
	}
}

//WithHijacker 劫持者
//部分option的设置失效，需自行实现，不能阻塞，只能传递[]byte
func WithHijacker(h func([]byte, WriterFunc) error) Option {
	return func(o *Options) {
		if h == nil {
			o.Logger.Fatal("WithHijacker：Hijacker is nil")
		}
		o.Hijacker = h
	}
}

//WithIntletHook 入口拦截器
func WithIntletHook(chain ...func([]byte, WriterFunc) ([]byte, error)) Option {
	return func(o *Options) {
		if len(chain) == 0 {
			o.Logger.Fatal("WithIntletHook：IntletHook is nil")
		}
		o.IntletHook = append(o.IntletHook, chain...)
	}
}

//WithOutletHook 出口拦截器
func WithOutletHook(chain ...func([]byte, WriterFunc) ([]byte, error)) Option {
	return func(o *Options) {
		if len(chain) == 0 {
			o.Logger.Fatal("WithOutletHook：OutletHook is nil")
		}
		o.OutletHook = append(o.OutletHook, chain...)
	}
}

//WithBreaker 熔断器
func WithBreaker(allow func() error, success, failed func()) Option {
	return func(o *Options) {
		if allow != nil && success != nil && failed != nil {
			o.AllowRequest = allow
			o.MarkSuccess = success
			o.MarkFailed = failed
		}
	}
}

/*
//WithRegistry 服务的注册和发现
func WithRegistry(ctx context.Context, info NodeInfo, r IRegistry) Option {
	return func(o *Options) {
		o.Registry = r
		buf, err := json.Marshal(info)
		if err != nil {
			panic(err.Error())
		}
		err = r.Registry(buf)
		if err != nil {
			panic(err.Error())
		}
		if len(info.TCPPort) > 0 {
			o.tcpServer = NewTCPServer(ctx, info.TCPPort, o.serveRequest, o.Logger)
			go o.tcpServer.Run()
		}
	}
}
*/

// https://mp.weixin.qq.com/s/EvkMQCPwg-B0fZonpwXodg
