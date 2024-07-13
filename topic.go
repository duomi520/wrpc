package wrpc

import (
	"github.com/duomi520/utils"
	"io"
)

// Topic 主题
type Topic struct {
	*Service
	Name         string
	audienceList *utils.CopyOnWriteList
}

func (t *Topic) add(id int64, w io.Writer) {
	t.audienceList.Add(Conn{id: id, w: w})
}
func (t *Topic) remove(id int64) {
	t.audienceList.Remove(Conn{id: id}.Equal)
}
func (t *Topic) traverse(data []byte) {
	all := t.audienceList.List()
	for _, v := range all {
		_, err := v.(Conn).w.Write(data)
		if err != nil {
			t.Logger.Error(err.Error())
		}
	}
}

// Broadcast 广播
func (t *Topic) Broadcast(data []byte) error {
	id, err := t.snowFlakeID.NextID()
	if err != nil {
		return err
	}
	f := NewFrame(utils.StatusBroadcast16, id, t.Name)
	buf := utils.AllocBuffer()
	defer utils.FreeBuffer(buf)
	//TODO 优化少copy一次
	err = FrameEncode(f, utils.MetaDict[string]{}, data, buf, t.Encoder)
	if err != nil {
		return err
	}
	t.traverse(buf.Bytes())
	return nil
}
