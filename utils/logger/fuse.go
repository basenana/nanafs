package logger

import (
	"bytes"
	"go.uber.org/zap"
	"log"
	"strings"
)

const (
	defaultFuseLoggerBuf = 1024
	fuseLogMessageDelim  = '\n'
)

func NewFuseLogger() *log.Logger {
	if root == nil {
		return nil
	}
	adp := &fuseLogAdaptor{
		b: bytes.NewBuffer(make([]byte, defaultFuseLoggerBuf)),
		l: root.Named("fuse"),
	}
	return log.New(adp, "", log.Ldate|log.Lmicroseconds)
}

type fuseLogAdaptor struct {
	b *bytes.Buffer
	l *zap.SugaredLogger
}

func (f *fuseLogAdaptor) Write(p []byte) (n int, err error) {
	n, err = f.b.Write(p)
	if n == 0 {
		return
	}

	if p[n-1] == fuseLogMessageDelim {
		f.logLine()
	}
	return
}

func (f *fuseLogAdaptor) logLine() {
	var (
		line   string
		logErr error
	)
	line, logErr = f.b.ReadString(fuseLogMessageDelim)
	if logErr != nil {
		f.l.Errorf("read log buf failed: %s", logErr.Error())
		return
	}
	f.l.Info(strings.TrimSpace(line))
}
