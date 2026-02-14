package api

import (
	"bytes"
	"context"
)

func ReadAllContent(ctx context.Context, resp *Response) (string, error) {
	var (
		contentBuf = &bytes.Buffer{}
		err        error
	)

Waiting:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break Waiting
		case err = <-resp.Error():
			if err != nil {
				break Waiting
			}
		case delta, ok := <-resp.Deltas():
			if !ok {
				break Waiting
			}
			if delta.Content != "" {
				contentBuf.WriteString(delta.Content)
			}
		}
	}

	return contentBuf.String(), err
}
