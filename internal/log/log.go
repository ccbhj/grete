package log

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
)

var (
	logOut = os.Stdout
	dbg    = true
	logger *log.Logger
)

func init() {
	logger = log.New(logOut, "", log.Lmicroseconds)
}

// Debug only print log when debug toggle is on
func Debug(format string, args ...any) {
	if !dbg {
		return
	}
	logger.Printf(logFormat(format, args...))
}

func logFormat(format string, args ...any) string {
	_, file, lineno, _ := runtime.Caller(2)
	file = path.Base(file)
	s := fmt.Sprintf("%s:%d|"+format, append([]any{file, lineno}, args...)...)
	return s
}

// L print log
func L(format string, args ...any) {
	logger.Printf(logFormat(format, args...))
}

// L print log with a prefix
func LP(prefix, format string, args ...any) {
	logger.Printf(logFormat("["+prefix+"] "+format, args...))
}

// BugOn assert 'exp' to be true, or panic when debug toggle is on
func BugOn(exp bool, format string, args ...any) {
	if dbg && !exp {
		log.Panicf("[BUG] "+format, args...)
	}
}
