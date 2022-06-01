package wrpc

//IBreaker 熔断器接口
type IBreaker interface {
	AllowRequest() bool
	Metrics(bool)
	SetBreakerPass()
}

//回路状态
//关闭状态：服务正常，并维护一个失败率统计，当失败率达到阀值时，转到开启状态
//开启状态：服务异常，一段时间之后，进入半开启状态
//半开启装态：这时熔断器只允许一个请求通过. 当该请求调用成功时, 熔断器恢复到关闭状态. 若该请求失败, 熔断器继续保持打开状态, 接下来的请求被禁止通过

//CircuitBreaker 回路
type CircuitBreaker struct {
}

//NewCircuitBreaker 新加回路
func NewCircuitBreaker() *CircuitBreaker {
	cb := &CircuitBreaker{}
	return cb
}

//AllowRequest 判断是否允许通过
func (circuit *CircuitBreaker) AllowRequest() bool {
	return true
}

//Metrics 保护算法
func (circuit *CircuitBreaker) Metrics(b bool) {
}

//SetBreakerPass 强制将熔断切换到closed状态，快速回复系统正常运转
func (cb *CircuitBreaker) SetBreakerPass() {
}

// http://ldaysjun.com/2019/05/07/Distributed/1.4/
// https://github.com/afex/hystrix-go
// https://mp.weixin.qq.com/s/DGRnUhyv6SS_E36ZQKGpPA
// https://blog.51cto.com/u_15175878/2774901
