package utils

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

var idGenerator *snowflake.Node

func init() {
	var err error
	idGenerator, err = snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func GenerateNewID() int64 {
	return idGenerator.Generate().Int64()
}
