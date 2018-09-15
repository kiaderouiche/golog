package golog

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/getlantern/errors"
	"github.com/getlantern/ops"

	"github.com/stretchr/testify/assert"
)

var (
	expectedLog      = "SEVERITY myprefix: golog_test.go:999 Hello world\nSEVERITY myprefix: golog_test.go:999 Hello true [cvarA=a cvarB=b op=name root_op=name]\n"
	expectedErrorLog = `ERROR myprefix: golog_test.go:999 Hello world [cvarC=c cvarD=d error=Hello %v error_location=github.com/getlantern/golog.TestError (golog_test.go:999) error_text=Hello world error_type=errors.Error op=name root_op=name]
ERROR myprefix: golog_test.go:999   at github.com/getlantern/golog.TestError (golog_test.go:999)
ERROR myprefix: golog_test.go:999   at testing.tRunner (testing.go:999)
ERROR myprefix: golog_test.go:999   at runtime.goexit (asm_amd999.s:999)
ERROR myprefix: golog_test.go:999 Caused by: world
ERROR myprefix: golog_test.go:999   at github.com/getlantern/golog.errorReturner (golog_test.go:999)
ERROR myprefix: golog_test.go:999   at github.com/getlantern/golog.TestError (golog_test.go:999)
ERROR myprefix: golog_test.go:999   at testing.tRunner (testing.go:999)
ERROR myprefix: golog_test.go:999   at runtime.goexit (asm_amd999.s:999)
ERROR myprefix: golog_test.go:999 Hello true [cvarA=a cvarB=b cvarC=c error=%v %v error_location=github.com/getlantern/golog.TestError (golog_test.go:999) error_text=Hello true error_type=errors.Error op=name999 root_op=name999]
ERROR myprefix: golog_test.go:999   at github.com/getlantern/golog.TestError (golog_test.go:999)
ERROR myprefix: golog_test.go:999   at testing.tRunner (testing.go:999)
ERROR myprefix: golog_test.go:999   at runtime.goexit (asm_amd999.s:999)
ERROR myprefix: golog_test.go:999 Caused by: Hello
ERROR myprefix: golog_test.go:999   at github.com/getlantern/golog.TestError (golog_test.go:999)
ERROR myprefix: golog_test.go:999   at testing.tRunner (testing.go:999)
ERROR myprefix: golog_test.go:999   at runtime.goexit (asm_amd999.s:999)
`
	expectedTraceLog = "TRACE myprefix: golog_test.go:999 Hello world\nTRACE myprefix: golog_test.go:999 Hello true\nTRACE myprefix: golog_test.go:999 Gravy\nTRACE myprefix: golog_test.go:999 TraceWriter closed due to unexpected error: EOF\n"
	expectedStdLog   = expectedLog
)

var (
	replaceNumbers = regexp.MustCompile("[0-9]+")
)

func init() {
	Impl = goLogLogger
	ops.SetGlobal("global", "shouldn't show up")
}

func expected(severity string, log string) string {
	return strings.Replace(log, "SEVERITY", severity, -1)
}

func normalized(log string) string {
	return replaceNumbers.ReplaceAllString(log, "999")
}

func TestReport(t *testing.T) {
	SetOutputs(ioutil.Discard, ioutil.Discard)
	OnFatal(func(err error) {
		// ignore (prevents test from exiting)
	})

	errors := 0
	fatals := 0
	RegisterReporter(func(err error, linePrefix string, severity Severity, ctx map[string]interface{}) {
		switch severity {
		case ERROR:
			errors++
		case FATAL:
			fatals++
		}
	})
	l := LoggerFor("reporting")
	l.Error("Some error")
	l.Fatal("Fatal error")
	assert.Equal(t, 1, errors)
	assert.Equal(t, 1, fatals)
}

func TestDebug(t *testing.T) {
	out := newBuffer()
	SetOutputs(ioutil.Discard, out)
	l := LoggerFor("myprefix")
	l.Debug("Hello world")
	defer ops.Begin("name").Set("cvarA", "a").Set("cvarB", "b").End()
	l.Debugf("Hello %v", true)
	assert.Equal(t, expected("DEBUG", expectedLog), out.String())
}

func TestError(t *testing.T) {
	out := newBuffer()
	SetOutputs(out, ioutil.Discard)
	l := LoggerFor("myprefix")
	ctx := ops.Begin("name").Set("cvarC", "c")
	err := errorReturner()
	err1 := errors.New("Hello %v", err)
	err2 := errors.New("Hello")
	ctx.End()
	l.Error(err1)
	defer ops.Begin("name2").Set("cvarA", "a").Set("cvarB", "b").End()
	l.Errorf("%v %v", err2, true)
	t.Log(out.String())
	assert.Equal(t, expectedErrorLog, out.String())
}

func errorReturner() error {
	defer ops.Begin("name").Set("cvarD", "d").End()
	return errors.New("world")
}

func TestAsStdLogger(t *testing.T) {
	out := newBuffer()
	SetOutputs(out, ioutil.Discard)
	l := LoggerFor("myprefix")
	stdlog := l.AsStdLogger()
	stdlog.Print("Hello world")
	defer ops.Begin("name").Set("cvarA", "a").Set("cvarB", "b").End()
	stdlog.Printf("Hello %v", true)
	assert.Equal(t, expected("ERROR", expectedStdLog), out.String())
}

// TODO: TraceWriter appears to have been broken since we added line numbers
// func TestTraceWriter(t *testing.T) {
// 	originalTrace := os.Getenv("TRACE")
// 	err := os.Setenv("TRACE", "true")
// 	if err != nil {
// 		t.Fatalf("Unable to set trace to true")
// 	}
// 	defer func() {
// 		if err := os.Setenv("TRACE", originalTrace); err != nil {
// 			t.Fatalf("Unable to set TRACE environment variable: %v", err)
// 		}
// 	}()
//
// 	out := newBuffer()
// 	SetOutputs(ioutil.Discard, out)
// 	l := LoggerFor("myprefix")
// 	trace := l.TraceOut()
// 	trace.Write([]byte("Hello world\n"))
// 	defer ops.Begin().Set("cvarA", "a").Set("cvarB", "b").End()
// 	trace.Write([]byte("Hello true\n"))
// 	assert.Equal(t, expected("TRACE", expectedStdLog), out.String())
// }

func newBuffer() *synchronizedbuffer {
	return &synchronizedbuffer{orig: &bytes.Buffer{}}
}

type synchronizedbuffer struct {
	orig  *bytes.Buffer
	mutex sync.RWMutex
}

func (buf *synchronizedbuffer) Write(p []byte) (int, error) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	return buf.orig.Write(p)
}

func (buf *synchronizedbuffer) String() string {
	buf.mutex.RLock()
	defer buf.mutex.RUnlock()
	return normalized(buf.orig.String())
}
