package wrpc

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBroadcast(t *testing.T) {
	Default()
	defer Stop()
	count := 0
	var wg sync.WaitGroup
	wg.Add(2)
	o := NewOptions()
	s := NewService(o)
	err := s.TCPServer(":4567")
	defer s.Stop()
	if err != nil {
		t.Fatal(err.Error())
	}
	topic := s.RegisterTopic("room")
	c1, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer c1.Close()
	c2, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer c2.Close()
	f := func(data []byte) error {
		count++
		if !strings.EqualFold(string(data), "Good!") {
			t.Fatal("data is not Good")
		}
		wg.Done()
		return nil
	}
	err = c1.Subscribe("room", f)
	defer c1.Unsubscribe("room")
	if err != nil {
		t.Fatal(err.Error())
	}
	err = c2.Subscribe("room", f)
	defer c2.Unsubscribe("room")
	if err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(400 * time.Millisecond)
	err = topic.Broadcast([]byte("Good!"))
	if err != nil {
		t.Fatal(err.Error())
	}
	wg.Wait()
	if count != 2 {
		t.Fatal("count is not 2")
	}
}
