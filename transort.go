package wrpc

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/duomi520/utils"
)

// DefaultDeadlineDuration IO超时
var DefaultDeadlineDuration = 10 * time.Second

// DefaultHeartbeatDuration 心跳包
var DefaultHeartbeatDuration time.Duration = time.Second * 5

// DefaultHeartbeatPeriodDuration 心跳包间距
var DefaultHeartbeatPeriodDuration time.Duration = (DefaultHeartbeatDuration * 9) / 10

// DefaultDelayOffDuration 延迟关闭时间
var DefaultDelayOffDuration = 5 * time.Second

type favContextKey string

var metadataKey favContextKey = "metadata"

// SetMeta
func SetMeta(ctx context.Context, m utils.MetaDict[string]) context.Context {
	return context.WithValue(ctx, metadataKey, m)
}

// GetMeta 取
func GetMeta(ctx context.Context) utils.MetaDict[string] {
	v := ctx.Value(metadataKey)
	if v != nil {
		return v.(utils.MetaDict[string])
	}
	return utils.MetaDict[string]{}
}

// RPCContext 上下文
type RPCContext struct {
	Metadata utils.MetaDict[string]
}

// Conn 连接
type Conn struct {
	id int64
	w  io.Writer
	//Send func([]byte) error
}

func (c Conn) Equal(obj any) bool {
	return c.id == obj.(Conn).id
}

// WriteTo
func (c Conn) WriteTo(p net.Buffers) {
	p.WriteTo(c.w)

}

/*
func (c Connect) warpSendb([]byte) error {
	return nil
}
*/
