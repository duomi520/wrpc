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
		reply, err := r.functionCall(v.key, nil, body)
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
2022/01/26 18:52:54  0 | 106 = 106 |
2022/01/26 18:52:54  1 | [8 10 12 14 16] = [8 10 12 14 16] |
2022/01/26 18:52:54  2 | 56 = 56 |
2022/01/26 18:52:54  3 | {0 7} = {0 7} |
2022/01/26 18:52:54  4 | <nil> = {0 0} | divide by zero
*/

func TestRPC(t *testing.T) {
	StartGuardian()
	defer StopGuardian()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	s := NewTCPServer(":4567", r.serveRequest, logger)
	go s.Run()
	rc1 := rpcResponseGet()
	rc2 := rpcResponseGet()
	rc3 := rpcResponseGet()
	defer func() {
		rpcResponsePut(rc1)
		rpcResponsePut(rc2)
		rpcResponsePut(rc3)
	}()
	c, err := NewTCPClient("127.0.0.1:4567", o)
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
	f1.MarshalBinary(json.Marshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	f2 := Frame{utils.StatusRequest16, 2, "计算.Divide", nil, Args{7, 8}}
	buf.reset()
	f2.MarshalBinary(json.Marshal, buf)
	if err := c.send(buf.bytes()); err != nil {
		t.Fatal(err)
	}
	f3 := Frame{utils.StatusRequest16, 3, "计算.Divide", nil, Args{9, 0}}
	buf.reset()
	f3.MarshalBinary(json.Marshal, buf)
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
[Info ] 2022-10-17 20:54:59 Pid: 82252
[Debug] 2022-10-17 20:54:59 TCP监听端口:4567
[Debug] 2022-10-17 20:54:59 TCP已初始化连接，等待客户端连接……
56 {0 7} divide by zero
[Debug] 2022-10-17 20:54:59 TCP监听端口关闭。
[Debug] 2022-10-17 20:54:59 TCP等待子协程关闭……
[Debug] 2022-10-17 20:55:04 127.0.0.1:60876 tcpServerReceive stop
[Debug] 2022-10-17 20:55:04 TCPServer关闭。
[Debug] 2022-10-17 20:55:04 127.0.0.1:4567 tcpClientReceive stop
*/
