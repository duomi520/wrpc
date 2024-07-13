package wrpc

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/duomi520/utils"
)

func TestFrame(t *testing.T) {
	encoder := func(a any, w io.Writer) error {
		return json.NewEncoder(w).Encode(a)
	}
	var meta utils.MetaDict[string]
	meta.Set("charset", "utf-8")
	var frames = []Frame{
		{utils.StatusRequest16, 1, utils.Hash64FNV1A("wang")},
		{utils.StatusResponse16, 2, utils.Hash64FNV1A("劳动节5.1")},
	}
	var metas = []utils.MetaDict[string]{{}, meta}
	var payloads = []string{"hi", "International Labour Day"}
	for i := range frames {
		var buf bytes.Buffer
		err := FrameEncode(frames[i], metas[i], payloads[i], &buf, encoder)
		if err != nil {
			t.Fatal(err.Error())
		}
		data := buf.Bytes()
		f, m, p, err := FrameUnmarshal(data, json.Unmarshal)
		if err != nil {
			t.Fatal(err.Error())
		}
		if f.Status != getStatus(data) {
			t.Errorf("1 expected %d got %d", f.Status, getStatus(data))
		}
		if !strings.EqualFold(payloads[i], p.(string)) {
			t.Errorf("2 expected %s got %s", payloads[i], p.(string))
		}
		if m.Len() != 0 {
			expected, _ := meta.Get("charset")
			got, _ := m.Get("charset")
			if !strings.EqualFold(expected, got) {
				t.Errorf("3 expected %s got %s", expected, got)
			}
		}
	}
}
