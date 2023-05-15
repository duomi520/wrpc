package wrpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/duomi520/utils"
)

func TestFrame(t *testing.T) {
	var meta utils.MetaDict
	meta.Set("charset", "utf-8")
	var tests = []Frame{
		{utils.StatusRequest16, 1, "wang", nil, "hi", nil},
		{utils.StatusResponse16, 2, "劳动节5.1", &meta, "International Labour Day", nil},
	}
	buf := bufferPool.Get().(*buffer)
	defer bufferPool.Put(buf)
	for i := range tests {
		err := tests[i].MarshalBinary(jsonMarshal, buf)
		if err != nil {
			t.Fatal(err.Error())
		}
		data := buf.bytes()
		f := &Frame{}
		l, err := f.UnmarshalHeader(data)
		if err != nil {
			t.Fatal(err.Error())
		}
		err = json.Unmarshal(data[l:], &f.Payload)
		if err != nil {
			t.Fatal(err.Error())
		}
		if !strings.EqualFold(f.ServiceMethod, tests[i].ServiceMethod) {
			t.Errorf("expected %s got %s", tests[i].ServiceMethod, f.ServiceMethod)
		}
		if !strings.EqualFold(f.Payload.(string), tests[i].Payload.(string)) {
			t.Errorf("expected %s got %s", tests[i].Payload, f.Payload)
		}
		if !strings.EqualFold(GetPayload(json.Unmarshal, data).(string), tests[i].Payload.(string)) {
			t.Errorf("expected %s got %s", tests[i].Payload.(string), GetPayload(json.Unmarshal, data).(string))
		}
		if tests[i].Status != GetStatus(data) {
			t.Errorf("expected %d got %d", tests[i].Status, GetStatus(data))
		}
		if f.Metadata != nil {
			expected, _ := meta.Get("charset")
			got, _ := f.Metadata.Get("charset")
			if !strings.EqualFold(expected, got) {
				t.Errorf("expected %s got %s", expected, got)
			}
		}

	}
}
