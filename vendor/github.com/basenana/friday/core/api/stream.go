package api

import (
	"bytes"
	"context"
)

func ReadAllContent(ctx context.Context, resp *Response) (string, error) {
	var (
		contentBuf = &bytes.Buffer{}
		answerBuf  = &bytes.Buffer{}
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
		case evt, ok := <-resp.Events():
			if !ok {
				break Waiting
			}

			if evt.Delta != nil && evt.Delta.Content != "" {
				contentBuf.WriteString(evt.Delta.Content)
			}

			if evt.Answer != nil && evt.Answer.Report != "" {
				answerBuf.WriteString(evt.Delta.Content)
			}
		}
	}

	var content = contentBuf.String()
	if content == "" {
		content = answerBuf.String()
	}

	return content, err
}

func CopyResponse(ctx context.Context, from *Response, to *Response) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-from.Events():
			if !ok {
				return nil
			}
			SendEvent(to, &evt)
		case err := <-from.Error():
			if err != nil {
				to.Fail(err)
				return err
			}
		}
	}
}
