package common

const (
	// CallTrigger process & source Plugin
	CallTrigger = "trigger"

	// CallListEntries and other Entry call is needed by mirror Plugin
	CallListEntries = "listEntries"
	CallAddEntry    = "addEntry"
	CallUpdateEntry = "updateEntry"
	CallDeleteEntry = "deleteEntry"
)

type Request struct {
	CallType string
	WorkPath string
	Entry    Entry
}

func NewRequest() *Request {
	return &Request{}
}

type Response struct {
	IsSucceed bool
	Entries   []Entry
}

func NewResponse() *Response {
	return &Response{}
}
