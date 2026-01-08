package utils

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"

	"github.com/davecgh/go-spew/spew"
)

func ComputeHierarchyStructHash(parent, child interface{}, collisionCount *int32) string {
	parentSpecHasher := fnv.New32a()
	childSpecHasher := fnv.New32a()
	DeepHashObject(parentSpecHasher, parent)
	DeepHashObject(childSpecHasher, child)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(collisionCountBytes, uint32(*collisionCount))
		_, _ = parentSpecHasher.Write(collisionCountBytes)
		_, _ = childSpecHasher.Write(collisionCountBytes)
	}

	merge := parentSpecHasher.Sum32() ^ childSpecHasher.Sum32()
	return fmt.Sprint(merge)
}

func ComputeStructHash(template interface{}, collisionCount *int32) string {
	templateSpecHasher := fnv.New32a()
	DeepHashObject(templateSpecHasher, template)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(collisionCountBytes, uint32(*collisionCount))
		_, _ = templateSpecHasher.Write(collisionCountBytes)
	}

	return fmt.Sprint(templateSpecHasher.Sum32())
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	_, _ = printer.Fprintf(hasher, "%#v", objectToWrite)
}
