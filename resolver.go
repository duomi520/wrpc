package wrpc

import "context"

//NodeInstance 节点信息
type NodeInstance struct {
	Id       int64
	Name     string
	Endpoint string
}

//IRegistry 服务注册
type IRegistrar interface {
	//注册实例
	Register(context.Context, *NodeInstance) error
	//解除注册实例
	Deregister(context.Context, *NodeInstance) error
}

// Watcher is service watcher.
type Watcher interface {
	Next() ([]*NodeInstance, error)
	Stop() error
}

//IDiscovery 服务发现
type IDiscovery interface {
	// 根据 serviceName 直接拉取实例列表
	GetService(context.Context, string) ([]*NodeInstance, error)
	// 根据 serviceName 阻塞式订阅一个服务的实例列表信息
	Watch(context.Context, string) (Watcher, error)
}
