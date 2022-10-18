package wrpc

import (
	"github.com/duomi520/utils"
)

//Topic 主题
type Topic struct {
	*Service
	Name         string
	audienceList *utils.LockList
}

func (t *Topic) add(id int64, s func([]byte) error) {
	t.audienceList.Add(connect{Id: id, send: s})
}
func (t *Topic) remove(id int64) {
	t.audienceList.Remove(connect{Id: id}.equal)
}
func (t *Topic) traverse(data []byte) {
	all := t.audienceList.List()
	for _, v := range all {
		v.(connect).send(data)
	}
}

//Broadcast 广播
func (t *Topic) Broadcast(data []byte) error {
	id, err := t.snowFlakeID.NextID()
	if err != nil {
		return err
	}
	f := Frame{Status: utils.StatusBroadcast16, Seq: id, ServiceMethod: t.Name, Payload: data}
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	err = f.MarshalBinary(t.Marshal, buf)
	if err != nil {
		return err
	}
	t.traverse(buf.bytes())
	return nil
}
