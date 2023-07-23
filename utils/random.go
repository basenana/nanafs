/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/bwmarrin/snowflake"
	"math/rand"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	randomString := hex.EncodeToString(bytes)
	return randomString[:length], nil
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
	hostname, err := os.Hostname()
	if err != nil {
		hostname = RandStringRunes(8)
	}

	h := sha256.New()
	h.Write([]byte(hostname))
	hashedHostName := h.Sum(nil)

	idTmp := make([]byte, 8)
	copy(idTmp, hashedHostName)
	nodeId := int64(binary.BigEndian.Uint64(idTmp))

	idGenerator, err = snowflake.NewNode(nodeId)
	if err != nil {
		panic(err)
	}
}

func GenerateNewID() int64 {
	return idGenerator.Generate().Int64()
}
