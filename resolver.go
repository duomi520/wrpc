package wrpc

import (
	"sync"
	"time"
)

//NodeInfo 节点信息
type NodeInfo struct {
	Id       int64
	Address  string
	HTTPPort string
	TCPPort  string
}
type Node struct {
	NodeInfo
	*Client
}

type NodeList struct {
	sync.Mutex
	conns  []Node
	weight []int
}

//Resolver 解析器
type Resolver struct {
	serviceMethodMap sync.Map
	//	clientList *LockList
	Balancer func()
}

func (r *Resolver) run() {
	for {
		select {
		default:
			time.Sleep(time.Second)
		}
	}
}

//Pick
func (r *Resolver) Pick(serviceMethod string) (*connect, error) {
	return nil, nil
}

/*
//IRegistry 服务的注册和发现
type IRegistry interface {
	Registry([]byte) error
	Put(string) error
	GetID(string) []int64
	GetInfo(int64) []byte
	Close()
}
*/

/*
//Balancer 平衡器
type Balancer interface {
	Pick(string, []int64) (int64, *Client)
	SetClientMap(int64, *Client)
}

//BalancerPoll 轮询
type BalancerPoll struct {
	sync.Mutex
	clientMap map[int64]*Client
	targetMap map[string]int
}

//NewBalancerPoll 新建
func NewBalancerPoll() *BalancerPoll {
	p := &BalancerPoll{}
	p.clientMap = make(map[int64]*Client)
	p.targetMap = make(map[string]int)
	return p
}

//SetClientMap s
func (p *BalancerPoll) SetClientMap(i int64, c *Client) {
	p.Lock()
	defer p.Unlock()
	p.clientMap[i] = c
}

//Pick 方法实现负载均衡算法逻辑
func (p *BalancerPoll) Pick(target string, list []int64) (int64, *Client) {
	var current int
	p.Lock()
	defer p.Unlock()
	current = p.targetMap[target]
	p.targetMap[target] = current + 1
	current = current % len(list)
	c := p.clientMap[list[current]]
	return list[current], c
}
*/
// Balancer 选择器通过选择提供负载均衡机制
// https://zhuanlan.zhihu.com/p/219672934?utm_source=wechat_session
// https://studygolang.com/articles/28742#reply0
// https://gocn.vip/topics/11340
// https://blog.csdn.net/liyunlong41/article/details/103724258
// https://www.jianshu.com/p/5c8e5548abdb
// https://blog.csdn.net/yimin_tank/article/details/82759501
// 轮询 – 平均分配使得每一个后端（服务）消耗相同的资源
// 带权轮询 – 根据后端的处理能力来分配权重
// 最少连接数 – 负载被分发至当前最少连接的服务

// https://github.com/micro/go-micro
// https://www.cnblogs.com/lijianming180/p/12014054.html
