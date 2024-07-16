package wrpc

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"reflect"
	"time"

	"github.com/duomi520/utils"
)

//依赖路径 option->codec->transort->tcp->server and client

// SnowFlakeStartupTime
var SnowFlakeStartupTime int64 = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()

// defaultProtocolMagicNumber 缺省协议头
const defaultProtocolMagicNumber uint32 = 2299

// MethodInfo 方法
type MethodInfo struct {
	method    reflect.Method
	val       reflect.Value
	argsType  reflect.Type
	replyType reflect.Type
	stream    *Stream
}

// Options 配置
type Options struct {
	ProtocolMagicNumber uint32
	//ID生成器
	snowFlakeID *utils.SnowFlakeID
	//本地服务
	methodMap map[uint64]*MethodInfo
	//编码器
	Marshal func(any) ([]byte, error)
	Encoder func(any, io.Writer) error
	//解码器
	Unmarshal func([]byte, any) error
	//服务端拦截器
	WarpHandler func(func([]byte, io.Writer) error) func([]byte, io.Writer) error
	//客服端包装发送
	WarpSend func(func(context.Context, Frame, any) error) func(context.Context, Frame, any) error
	//日志
	Logger *slog.Logger
}

// Option 选项赋值
type Option func(*Options)

// NewOptions 创建并返回一个配置：接收Option函数类型的不定向参数列表
func NewOptions(opts ...Option) *Options {
	o := Options{}
	//设置默认值
	o.ProtocolMagicNumber = defaultProtocolMagicNumber
	o.snowFlakeID = utils.NewSnowFlakeID(0, SnowFlakeStartupTime)
	o.methodMap = make(map[uint64]*MethodInfo, 256)
	o.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	o.Marshal = json.Marshal
	o.Encoder = func(a any, w io.Writer) error {
		return json.NewEncoder(w).Encode(a)
	}
	o.Unmarshal = json.Unmarshal
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

// WithLogger 日志
func WithLogger(l *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// WithCodec 编码
func WithCodec(m func(any) ([]byte, error), e func(any, io.Writer) error, um func([]byte, any) error) Option {
	return func(o *Options) {
		if m != nil && e != nil && um != nil {
			o.Marshal = m
			o.Encoder = e
			o.Unmarshal = um
		}
	}
}

// WithWarpHandler 服务端拦截器
func WithWarpHandler(w func(func([]byte, io.Writer) error) func([]byte, io.Writer) error) Option {
	return func(o *Options) {
		if w != nil {
			o.WarpHandler = w
		}
	}
}

// WarpSend 客服端包装发送
func WarpSend(s func(func(context.Context, Frame, any) error) func(context.Context, Frame, any) error) Option {
	return func(o *Options) {
		if s != nil {
			o.WarpSend = s
		}
	}
}
