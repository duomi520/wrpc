package wrpc

import (
	"context"
	"time"

	"github.com/duomi520/utils"
)

//DefaultDeadlineDuration IO超时
const DefaultDeadlineDuration = 10 * time.Second

//DefaultHeartbeatDuration 心跳包
const DefaultHeartbeatDuration time.Duration = time.Second * 5

//DefaultHeartbeatPeriodDuration 心跳包间距
const DefaultHeartbeatPeriodDuration time.Duration = (DefaultHeartbeatDuration * 9) / 10

type favContextKey string

var metadataKey favContextKey = "metadata"

func MetadataContext(ctx context.Context, m *utils.MetaDict) context.Context {
	return context.WithValue(ctx, metadataKey, m)
}
func GetMetadata(ctx context.Context) *utils.MetaDict {
	return ctx.Value(metadataKey).(*utils.MetaDict)
}
