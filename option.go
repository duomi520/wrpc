package wrpc

import (
	"encoding/json"
	"github.com/duomi520/utils"
	"time"
)

var snowFlakeStartupTime int64 = time.Date(2022, time.June, 1, 0, 0, 0, 0, time.UTC).UnixNano()

//Options 配置
type Options struct {
	snowFlakeID         *utils.SnowFlakeID
	Marshal             func(any) ([]byte, error)
	Unmarshal           func([]byte, any) error
	ProtocolMagicNumber uint32
	//验证
	Validator utils.IValidator
	//熔断器
	Breaker IBreaker
	//平衡器
	Balancer func([]int) int
	//Registry  IRegistry
	Logger utils.ILogger
}

//Option 选项赋值
type Option func(*Options)

//NewOptions 创建并返回一个配置：接收Option函数类型的不定向参数列表
func NewOptions(opts ...Option) *Options {
	o := Options{}
	//设置默认值
	o.snowFlakeID = utils.NewSnowFlakeID(0, snowFlakeStartupTime)
	o.Logger, _ = utils.NewWLogger(utils.FatalLevel, "")
	o.Marshal = json.Marshal
	o.Unmarshal = json.Unmarshal
	o.ProtocolMagicNumber = defaultProtocolMagicNumber
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

//WithValidator 验证
func WithValidator(v utils.IValidator) Option {
	return func(o *Options) {
		o.Validator = v
	}
}

//WithLogger 日志
func WithLogger(l utils.ILogger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

//WithCodec 编码
func WithCodec(m func(any) ([]byte, error), um func([]byte, any) error) Option {
	return func(o *Options) {
		if m != nil && um != nil {
			o.Marshal = m
			o.Unmarshal = um
		}
	}
}

//WithRegistry 服务的注册和发现
/*
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
//WithBreaker 熔断器
func WithBreaker(b IBreaker) Option {
	return func(o *Options) {
		o.Breaker = b
	}
}
