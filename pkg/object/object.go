package object

type Object interface {
	GetObjectMeta() *Metadata
	GetExtendData() *ExtendData
	GetCustomColumn() *CustomColumn
}
