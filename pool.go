package wrpc

import "sync"

var rpcResponsePool sync.Pool

// AllocRPCResponse 从池里取一个
func AllocRPCResponse() *rpcResponse {
	v := rpcResponsePool.Get()
	if v != nil {
		v.(*rpcResponse).Done = make(chan struct{})
		return v.(*rpcResponse)
	}
	var r rpcResponse
	r.Done = make(chan struct{})
	return &r
}

// FreeRPCResponse 还一个到池里
func FreeRPCResponse(r *rpcResponse) {
	r.id = 0
	r.client = nil
	r.reply = nil
	close(r.Done)
	r.Error = nil
	rpcResponsePool.Put(r)
}