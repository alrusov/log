/*
Package log implements a simple library for message logging
*/
package log

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alrusov/misc"
	"github.com/alrusov/panic"
)

//----------------------------------------------------------------------------------------------------------------------------//

// Level -- level of the logging
type Level int

const (
	// EMERG -- system is unusable
	EMERG Level = iota
	// ALERT -- action must be taken immediately
	ALERT
	// CRIT -- critical conditions
	CRIT
	// ERR -- error conditions
	ERR
	// WARNING -- warning conditions
	WARNING
	// NOTICE -- normal but significant condition
	NOTICE
	// INFO -- informational
	INFO
	// DEBUG -- debug-level messages
	DEBUG
	// TRACE1 -- trace 1
	TRACE1
	// TRACE2 -- trace 2
	TRACE2
	// TRACE3 -- trace 3
	TRACE3
	// TRACE4 -- trace 4
	TRACE4
	// UNKNOWN -- unknown level
	UNKNOWN
)

type logLevelDef struct {
	code      Level
	name      string
	shortName string
}

const (
	logFuncNameNone int = iota
	logFuncNameShort
	logFuncNameFull
)

const (
	beforeFileBufSize = 500
	lastBufSize       = 10
)

var (
	logLevels = []logLevelDef{
		{EMERG, "EMERG", "EM"},
		{ALERT, "ALERT", "AL"},
		{CRIT, "CRIT", "CR"},
		{ERR, "ERR", "ER"},
		{WARNING, "WARNING", "WA"},
		{NOTICE, "NOTICE", "NO"},
		{INFO, "INFO", "IN"},
		{DEBUG, "DEBUG", "DE"},
		{TRACE1, "TRACE1", "T1"},
		{TRACE2, "TRACE2", "T2"},
		{TRACE3, "TRACE3", "T3"},
		{TRACE4, "TRACE4", "T4"},
		{UNKNOWN, "UNKNOWN", "??"},
	}

	consoleWriter io.Writer

	enabled   = true
	active    = true
	firstTime = true

	dumpFileName  = "unsaved.log"
	beforeFileBuf = []string{}

	lastBuf = []string{}

	currentLogLevel = DEBUG

	logFuncName = logFuncNameNone

	logLock = new(sync.Mutex)

	localTime     = false
	lastWriteDate string

	fileDirectory   string
	fileNamePattern string
	fileName        string
	file            *os.File

	writerBufSize     = 0
	writer            *bufio.Writer
	writerMutex       = new(sync.Mutex)
	writerFlushPeriod = 0

	maxLen = 0

	pid int
)

// ChangeLevelAlertFunc --
type ChangeLevelAlertFunc func(old Level, new Level)

var (
	alertSubscriberID = int64(0)
	alertSubscribers  = map[int64]ChangeLevelAlertFunc{}
)

//----------------------------------------------------------------------------------------------------------------------------//

func init() {
	pid = os.Getpid()
	misc.AddExitFunc("log.exit", exit, nil)

	consoleWriter = &ConsoleWriter{}

	log.SetFlags(0)
	log.SetOutput(&stdLogWriter{})

	dumpFileName, _ = misc.AbsPath("@" + misc.AppName() + "_" + dumpFileName)

	go writerFlusher()
}

//----------------------------------------------------------------------------------------------------------------------------//

func now() time.Time {
	t := time.Now()

	if !localTime {
		return t.UTC()
	}

	return t
}

//----------------------------------------------------------------------------------------------------------------------------//

type stdLogWriter struct{}

func (l *stdLogWriter) Write(p []byte) (int, error) {
	MessageEx(1, ERR, nil, "", strings.TrimSpace(string(p)))
	return len(p), nil
}

// ConsoleWriter --
type ConsoleWriter struct{}

func (l *ConsoleWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return len(p), nil
}

// Testwriter --
type Testwriter struct {
	stream *testing.T
}

func (l *Testwriter) Write(p []byte) (n int, err error) {
	l.stream.Log(string(p))
	return len(p), nil
}

// SetConsoleWriter --
func SetConsoleWriter(writer io.Writer) {
	consoleWriter = writer
}

// SetTestWriter --
func SetTestWriter(stream *testing.T) {
	SetConsoleWriter(&Testwriter{stream: stream})
}

//----------------------------------------------------------------------------------------------------------------------------//

func writerFlush() {
	if writer != nil {
		writerMutex.Lock()
		writer.Flush()
		writerMutex.Unlock()
	}
}

func exit(code int, p interface{}) {
	Message(INFO, "Log file closed")

	if len(beforeFileBuf) > 0 {
		fd, err := os.OpenFile(dumpFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			for _, s := range beforeFileBuf {
				fd.Write([]byte(s))
			}
			fd.Close()
		}
	}

	active = false

	writerFlush()

	if file != nil {
		file.Close()
	}
}

func writerFlusher() {
	defer panic.SaveStackToLog()

	var period int
	lastFlushDate := ""

	for {
		if writerFlushPeriod == 0 {
			period = 1
		} else {
			period = writerFlushPeriod
		}

		if !misc.Sleep(time.Duration(period) * time.Second) {
			break
		} else {
			dt := now().Format(misc.DateFormatRev)
			if lastFlushDate != "" && dt != lastFlushDate {
				Message(-1*INFO, "Have a nice day")
			}
			lastFlushDate = dt
			writerFlush()
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

// Enable --
func Enable() {
	enabled = true
}

// Disable --
func Disable() {
	enabled = false
}

//----------------------------------------------------------------------------------------------------------------------------//

// GetLastLog --
func GetLastLog() []string {
	return lastBuf
}

//----------------------------------------------------------------------------------------------------------------------------//

// MaxLen --
func MaxLen(ln int) int {
	n := maxLen
	maxLen = ln
	return n
}

//----------------------------------------------------------------------------------------------------------------------------//

// Str2Level --
func Str2Level(levelName string) (level Level, ok bool) {
	level = UNKNOWN
	ok = false

	for _, def := range logLevels {
		if (levelName == def.name) || (levelName == def.shortName) {
			level = def.code
			ok = true
			break
		}
	}
	return level, ok
}

// GetLogLevels -- get log level list
func GetLogLevels() []string {
	list := make([]string, UNKNOWN)

	for _, def := range logLevels {
		if def.code < UNKNOWN {
			list[def.code] = def.name
		}
	}

	return list
}

// GetLogLevelName -- get log level names
func GetLogLevelName(level Level) (short string, long string) {
	return logLevels[level].shortName, logLevels[level].name
}

// GetCurrentLogLevel -- get log level
func GetCurrentLogLevel() (level Level, short string, long string) {
	level = currentLogLevel
	short, long = GetLogLevelName(currentLogLevel)
	return
}

// SetCurrentLogLevel -- set log level
func SetCurrentLogLevel(levelName string, logFunc string) (Level, error) {
	switch logFunc {
	case "short":
		logFuncName = logFuncNameShort
	case "full":
		logFuncName = logFuncNameFull
	case "none":
		fallthrough
	case "":
		fallthrough
	default:
		logFuncName = logFuncNameNone
	}

	level, ok := Str2Level(levelName)
	if !ok {
		msg := fmt.Sprintf(`Invalid log level "%s", left unchanged "%s" `, levelName, logLevels[currentLogLevel].name)
		err := errors.New(msg)
		logger(0, WARNING, nil, msg)
		return currentLogLevel, err
	}

	logLock.Lock()
	if currentLogLevel != level {
		for _, f := range alertSubscribers {
			f(currentLogLevel, level)
		}
		logLock.Unlock()

		currentLogLevel = level
		logger(0, INFO, nil, `Current log level was set to "%s"`, logLevels[level].name)
	} else {
		logLock.Unlock()
	}

	return currentLogLevel, nil
}

//----------------------------------------------------------------------------------------------------------------------------//

// AddAlertFunc --
func AddAlertFunc(f ChangeLevelAlertFunc) int64 {
	logLock.Lock()
	defer logLock.Unlock()

	alertSubscriberID++
	alertSubscribers[alertSubscriberID] = f
	return alertSubscriberID
}

// DelAlertFunc --
func DelAlertFunc(id int64) {
	logLock.Lock()
	defer logLock.Unlock()

	delete(alertSubscribers, id)
}

//----------------------------------------------------------------------------------------------------------------------------//

// SetFile -- file for log
func SetFile(directory string, suffix string, useLocalTime bool, bufSize int, flushPeriod int) {
	if directory == "" {
		directory = "./logs/"
	}

	fileDirectory = directory
	localTime = useLocalTime
	writerBufSize = bufSize

	if flushPeriod > 0 {
		writerFlushPeriod = flushPeriod
	}

	if fileDirectory == "-" {
		fileNamePattern = "-"
	} else {
		if suffix != "" {
			suffix = "-" + suffix
		}
		fileDirectory, _ = misc.AbsPath(fileDirectory)
		fileNamePattern, _ = misc.AbsPath(fileDirectory + "/%s" + suffix + ".log")
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func writeToConsole(msg string) {
	if consoleWriter != nil {
		consoleWriter.Write([]byte(msg))
	}
}

func write(s string) {
	if file != nil {
		if writer != nil {
			writerMutex.Lock()
			writer.Write([]byte(s))
			writerMutex.Unlock()
		} else {
			file.Write([]byte(s))
		}
	}
}

func openLogFile(dt string) {

	if file != nil {
		if writer != nil {
			writerMutex.Lock()
			writer.Flush()
			writer = nil
			writerMutex.Unlock()
		}
		file.Close()
		file = nil
	}

	if _, err := os.Stat(fileDirectory); os.IsNotExist(err) {
		os.MkdirAll(fileDirectory, 0755)
	}

	fileName = fmt.Sprintf(fileNamePattern, dt)
	file, _ = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	os.Stderr.Close()
	os.Stderr, _ = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	cmd := ""
	for i := 0; i < len(os.Args); i++ {
		cmd += " " + os.Args[i]
	}
	cmd = strings.TrimSpace(cmd)

	t := misc.AppStartTime() // AppStartTime in UTC zone
	if localTime {
		t = t.Local()
	}

	ts := misc.BuildTime()
	if ts != "" {
		ts = " [" + ts + "Z]"
	}

	msg := fmt.Sprintf("[%d] %s *** %s %s%s was launched at %sZ with command line \"%s\"",
		pid,
		logLevels[INFO].shortName,
		misc.AppName(),
		misc.AppVersion(),
		ts,
		t.Format(misc.DateTimeFormatRev),
		cmd)

	if maxLen > 0 && maxLen < len(msg) {
		msg = msg[:maxLen]
	}
	msg += misc.EOS

	if file != nil {
		if writerBufSize > 0 {
			writer = bufio.NewWriterSize(file, writerBufSize)
		}

		write(msg)

		if len(beforeFileBuf) > 0 {
			for _, s := range beforeFileBuf {
				write(s)
			}
			beforeFileBuf = []string{}
		}

		os.Remove(dumpFileName)
	}

	if firstTime {
		firstTime = false
		writeToConsole(msg)
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func logger(stackShift int, level Level, replace *misc.Replace, message string, params ...interface{}) {
	if !enabled {
		return
	}

	logLock.Lock()
	defer logLock.Unlock()

	now := now()
	dt := now.Format(misc.DateFormatRev)
	tm := now.Format(misc.TimeFormatWithMS)

	var funcName string
	if (level == EMERG) || (logFuncName == logFuncNameFull) {
		funcName = " " + misc.GetFuncName(stackShift+1, false) + ":"
	} else if logFuncName == logFuncNameShort {
		funcName = " " + misc.GetFuncName(stackShift+1, true) + ":"
	} else {
		funcName = ""
	}

	levelName := ""
	if (level >= EMERG) && (level < UNKNOWN) {
		levelName = logLevels[level].shortName
	} else {
		levelName = fmt.Sprintf("?%d?", level)
	}

	format := fmt.Sprintf("[%d] %s %s %s%s %s", pid, levelName, dt, tm, funcName, message)
	text := fmt.Sprintf(format, params...)
	if maxLen > 0 && maxLen < len(text) {
		text = text[:maxLen]
	}

	if replace != nil {
		text = replace.Do(text)
	}

	text += misc.EOS

	if active {
		if fileNamePattern == "" {
			ln := len(beforeFileBuf)
			if ln > beforeFileBufSize {
			} else if ln < beforeFileBufSize {
				beforeFileBuf = append(beforeFileBuf, text)
			} else {
				beforeFileBuf = append(beforeFileBuf, "...")
			}
		} else if fileNamePattern != "-" {
			if (file == nil) || (lastWriteDate != dt) {
				openLogFile(dt)
			}

			if file != nil {
				write(text)
				lastWriteDate = dt
			} else {
				lastWriteDate = ""
			}
		}
	}

	if len(lastBuf) >= lastBufSize {
		lastBuf = lastBuf[1:]
	}
	lastBuf = append(lastBuf, text)

	writeToConsole(text)
}

// MessageEx -- add message to the log with custom shift
func MessageEx(shift int, level Level, replace *misc.Replace, message string, params ...interface{}) {
	if level <= currentLogLevel {
		if level < 0 {
			level = -level
		}
		logger(shift+1, level, replace, message, params...)
	}
}

// Message -- add message to the log
func Message(level Level, message string, params ...interface{}) {
	MessageEx(1, level, nil, message, params...)
}

// SecuredMessage -- add message to the log with securing
func SecuredMessage(level Level, replace *misc.Replace, message string, params ...interface{}) {
	MessageEx(1, level, replace, message, params...)
}

// MessageWithSource -- add message to the log with source
func MessageWithSource(level Level, source string, message string, params ...interface{}) {
	Message(level, "["+source+"] "+message, params...)
}

// SecuredMessageWithSource -- add message to the log with source & securing
func SecuredMessageWithSource(level Level, replace *misc.Replace, source string, message string, params ...interface{}) {
	SecuredMessage(level, replace, "["+source+"] "+message, params...)
}

//----------------------------------------------------------------------------------------------------------------------------//

// FileNamePattern --
func FileNamePattern() string {
	return fileNamePattern
}

// FileName --
func FileName() string {
	return fileName
}

//----------------------------------------------------------------------------------------------------------------------------//

// ServiceLogger --
type ServiceLogger struct{}

// Error --
func (l *ServiceLogger) Error(v ...interface{}) error {
	Message(ERR, fmt.Sprint(v...))
	return nil
}

// Warning --
func (l *ServiceLogger) Warning(v ...interface{}) error {
	Message(WARNING, fmt.Sprint(v...))
	return nil
}

// Info --
func (l *ServiceLogger) Info(v ...interface{}) error {
	Message(INFO, fmt.Sprint(v...))
	return nil
}

// Errorf --
func (l *ServiceLogger) Errorf(message string, a ...interface{}) error {
	Message(ERR, message, a)
	return nil
}

// Warningf --
func (l *ServiceLogger) Warningf(message string, a ...interface{}) error {
	Message(WARNING, message, a)
	return nil
}

// Infof --
func (l *ServiceLogger) Infof(message string, a ...interface{}) error {
	Message(INFO, message, a)
	return nil
}

//----------------------------------------------------------------------------------------------------------------------------//

// StdLogger --
func StdLogger(level string, message string, params ...interface{}) {
	nLevel, _ := Str2Level(level)
	Message(nLevel, message, params...)
}

//----------------------------------------------------------------------------------------------------------------------------//
