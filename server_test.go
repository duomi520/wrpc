package wrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

type Args struct {
	A, B int
}
type Quotient struct {
	Quo, Rem int
}
type Arith int

func (t *Arith) Add100(_ context.Context, args int) (int, error) {
	return args + 100, nil
}
func (t *Arith) Double(_ context.Context, args []int) ([]int, error) {
	var reply []int
	for i := range args {
		reply = append(reply, args[i]*2)
	}
	return reply, nil
}

func (t *Arith) Multiply(_ context.Context, args Args) (int, error) {
	return args.A * args.B, nil
}
func (t *Arith) Divide(_ context.Context, args Args) (Quotient, error) {
	var quo Quotient
	if args.B == 0 {
		return quo, errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return quo, nil
}

func TestFunctionCall(t *testing.T) {
	type table struct {
		key    string
		args   any
		expect any
		err    string
	}
	var tests = []table{
		{"计算.Add100", 6, 106, ""},
		{"计算.Double", []int{4, 5, 6, 7, 8}, []int{8, 10, 12, 14, 16}, ""},
		{"计算.Multiply", Args{7, 8}, 56, ""},
		{"计算.Divide", Args{7, 8}, Quotient{0, 7}, ""},
		{"计算.Divide", Args{9, 0}, Quotient{0, 0}, "divide by zero"},
	}
	o := NewOptions()
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Log(err.Error())
	}
	for index, v := range tests {
		body, _ := json.Marshal(&v.args)
		m := r.methodMap[v.key]
		reply, err := r.functionCall(1, m, nil, body)
		if err != nil {
			if !strings.EqualFold(err.Error(), v.err) {
				t.Fatalf("%2d | %s | %s \n", index, err.Error(), v.err)
			}
		} else {
			r1, _ := json.Marshal(reply)
			r2, _ := json.Marshal(v.expect)
			if !bytes.EqualFold(r1, r2) {
				t.Fatalf("%2d | %v | %v \n", index, reply, v.expect)
			}
		}
		log.Printf("%2d | %v = %v | %v\n", index, reply, v.expect, v.err)
	}
}

/*
2022/12/03 21:59:01  0 | 106 = 106 |
2022/12/03 21:59:01  1 | [8 10 12 14 16] = [8 10 12 14 16] |
2022/12/03 21:59:01  2 | 56 = 56 |
2022/12/03 21:59:01  3 | {0 7} = {0 7} |
2022/12/03 21:59:01  4 | <nil> = {0 0} | divide by zero
*/

func TestRPC(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	s := NewTCPServer(":4567", nil, r.serveRequest, logger)
	go s.Run()
	rc1 := rpcResponseGet()
	rc2 := rpcResponseGet()
	rc3 := rpcResponseGet()
	defer func() {
		rpcResponsePut(rc1)
		rpcResponsePut(rc2)
		rpcResponsePut(rc3)
	}()
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(logger)))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var reply1 int
	rc1.reply = &reply1
	var reply2, reply3 Quotient
	rc2.reply = &reply2
	rc3.reply = &reply3
	rc1.id, rc2.id, rc3.id = 1, 2, 3
	rc1.client, rc2.client, rc3.client = c, c, c
	c.callMap.Store(int64(1), rc1)
	c.callMap.Store(int64(2), rc2)
	c.callMap.Store(int64(3), rc3)
	f1 := Frame{utils.StatusRequest16, 1, "计算.Multiply", nil, Args{7, 8}}
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	f1.MarshalBinary(jsonMarshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	f2 := Frame{utils.StatusRequest16, 2, "计算.Divide", nil, Args{7, 8}}
	buf.reset()
	f2.MarshalBinary(jsonMarshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	f3 := Frame{utils.StatusRequest16, 3, "计算.Divide", nil, Args{9, 0}}
	buf.reset()
	f3.MarshalBinary(jsonMarshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		<-rc1.Done
		wg.Done()
	}()
	go func() {
		<-rc2.Done
		wg.Done()
	}()
	go func() {
		<-rc3.Done
		wg.Done()
	}()
	wg.Wait()
	fmt.Println(reply1, reply2, rc3.Error)
	s.Stop()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-12-03 21:58:35 TCPServer.Run：TCP监听端口:4567
[Debug] 2022-12-03 21:58:35 TCPServer.Run：TCP已初始化连接，等待客户端连接……
56 {0 7} divide by zero
[Debug] 2022-12-03 21:58:35 TCPServer.Stop：TCP监听端口关闭。
[Debug] 2022-12-03 21:58:35 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 21:58:40 TCPSession.tcpServerReceive: 127.0.0.1:60904 stop
[Debug] 2022-12-03 21:58:40 TCPServer.Run: TCPServer关闭。
[Debug] 2022-12-03 21:58:40 TCPSession.tcpClientReceive: 127.0.0.1:4567 stop
*/
func TestServerHook(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	intelA := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("A", b)
		return b, nil
	}
	intelB := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("B", b)
		return b, nil
	}
	outlet := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("O", b)
		return b, nil
	}
	o.IntletHook = append(o.IntletHook, intelA, intelB)
	o.OutletHook = append(o.OutletHook, outlet)
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	s := NewTCPServer(":4567", nil, r.serveRequest, logger)
	go s.Run()
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(logger)))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	rc := rpcResponseGet()
	defer rpcResponsePut(rc)
	var reply int
	rc.reply = &reply
	c.callMap.Store(int64(1), rc)
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	f := Frame{utils.StatusRequest16, 1, "计算.Add100", nil, 6}
	f.MarshalBinary(jsonMarshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	<-rc.Done
	fmt.Println(rc, reply)
	s.Stop()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-12-03 21:59:13 TCPServer.Run：TCP监听端口:4567
[Debug] 2022-12-03 21:59:13 TCPServer.Run：TCP已初始化连接，等待客户端连接……
A [33 0 0 0 16 0 1 0 0 0 0 0 0 0 31 0 31 0 232 174 161 231 174 151 46 65 100 100 49 48 48 54 10]
B [33 0 0 0 16 0 1 0 0 0 0 0 0 0 31 0 31 0 232 174 161 231 174 151 46 65 100 100 49 48 48 54 10]
O [35 0 0 0 17 0 1 0 0 0 0 0 0 0 31 0 31 0 232 174 161 231 174 151 46 65 100 100 49 48 48 49 48 54 10]
&{0 <nil> 0xc00009e4e8 0xc0000863c0 <nil>} 106
[Debug] 2022-12-03 21:59:13 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 21:59:13 TCPServer.Stop：TCP监听端口关闭。
[Debug] 2022-12-03 21:59:18 TCPSession.tcpClientReceive: 127.0.0.1:4567 stop
[Debug] 2022-12-03 21:59:18 TCPSession.tcpServerReceive: 127.0.0.1:60924 stop
[Debug] 2022-12-03 21:59:18 TCPServer.Run: TCPServer关闭。
*/
