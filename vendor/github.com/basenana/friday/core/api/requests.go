package api

import (
	"github.com/basenana/friday/core/memory"
	"github.com/basenana/friday/core/types"
)

type Request struct {
	Session     *types.Session
	UserMessage string
	ImageURLs   []string
	Memory      *memory.Memory
}

type Response struct {
	e   chan types.Event
	err chan error
}

func (r *Response) Events() <-chan types.Event {
	return r.e
}

func (r *Response) Fail(err error) {
	r.err <- err
}
func (r *Response) Error() <-chan error {
	return r.err
}

func (r *Response) Close() {
	close(r.e)
	close(r.err)
}

func NewResponse() *Response {
	return &Response{e: make(chan types.Event, 5), err: make(chan error, 0)}
}

func SendEvent(resp *Response, evt *types.Event, extraKV ...string) {
	var (
		ev = make(map[string]string)
	)
	if len(extraKV) > 1 {
		for i := 1; i < len(extraKV); i += 2 {
			ev[extraKV[i-1]] = extraKV[i]
		}
	}
	if len(ev) > 0 {
		evt.ExtraValue = ev
	}
	resp.e <- *evt
}
