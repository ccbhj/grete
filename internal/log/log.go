package log

import (
	"log"
	"os"
)

var (
	logOut = os.Stdout
	debug  = false
	logger *log.Logger
)

func init() {
	logger = log.New(logOut, "", log.Lshortfile|log.Lmicroseconds)
}

// Debug only print log when debug toggle is on
func Debug(format string, args ...any) {
	if !debug {
		return
	}
	logger.Printf(format, args...)
}

// L print log
func L(format string, args ...any) {
	logger.Printf(format, args...)
}

// L print log with a prefix
func LP(prefix, format string, args ...any) {
	logger.Printf("["+prefix+"] "+format, args...)
}

// BugOn assert 'exp' to be true, or panic when debug toggle is on
func BugOn(exp bool, format string, args ...any) {
	if debug && exp {
		log.Fatalf("[BUG] "+format, args...)
	}
}
