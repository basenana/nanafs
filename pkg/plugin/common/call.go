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
}

type Response struct {
	IsSucceed bool
}
