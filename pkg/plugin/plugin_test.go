package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"time"
)

type dummyPlugin struct{}

func (d dummyPlugin) Run(ctx context.Context, parent *types.Object, params map[string]string) (<-chan types.SimpleFile, error) {
	ch := make(chan types.SimpleFile, 1024)
	go func() {
		cnt := 0
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ch <- types.SimpleFile{
					Name:    fmt.Sprintf("dummp-file-%d", cnt),
					IsGroup: false,
					Source:  "dummy-source",
					Open: func() (io.ReadWriteCloser, error) {
						return &dummyFile{content: []byte(fmt.Sprintf("dummp-file-%d", cnt))}, nil
					},
				}
			}
			cnt += 1
		}
	}()
	return ch, nil
}

func (d dummyPlugin) Name() string {
	return "dummy-source"
}

func (d dummyPlugin) Type() types.PluginType {
	return types.PluginLibType
}

type dummyFile struct {
	content []byte
	offset  int
}

func (d *dummyFile) Read(p []byte) (n int, err error) {
	n = copy(p, d.content[d.offset:])
	d.offset += n
	return n, nil
}

func (d *dummyFile) Write(p []byte) (n int, err error) {
	if len(p) > len(d.content)-d.offset {
		newLen := d.offset + len(p)
		newContent := make([]byte, newLen)
		copy(newContent, d.content)
		d.content = newContent
	}
	n = copy(d.content[d.offset:], p)
	d.offset += n
	return n, nil
}

func (d *dummyFile) Close() error {
	return nil
}
