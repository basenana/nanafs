package logger

import (
	"fmt"
	"github.com/tickstep/library-go/time"
	"io"
	"io/ioutil"
)

//PrintReader 输出Reader
func PrintReader(r io.Reader) {
	b, _ := ioutil.ReadAll(r)
	fmt.Printf("%s\n", b)
}

// PrintArgs 输出字符串数组
func PrintArgs(w io.Writer, args ...string) {
	for k, arg := range args {
		io.WriteString(w, fmt.Sprintf("args[%d] = `%s`, ", k, arg))
	}
	w.Write([]byte{'\n'})
}

func TimePrefix() string {
	return "[" + time.BeijingTimeOption("Refer") + "]"
}
