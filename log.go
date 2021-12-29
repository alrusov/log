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
	// TIME -- execution time
	TIME
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

// FuncNameMode --
type FuncNameMode string

const (
	// FuncNameModeNone --
	FuncNameModeNone = FuncNameMode("none")
	// FuncNameModeShort --
	FuncNameModeShort = FuncNameMode("short")
	// FuncNameModeFull --
	FuncNameModeFull = FuncNameMode("full")
)

const (
	logFuncNameNone int = iota
	logFuncNameShort
	logFuncNameFull
)

const (
	beforeFileBufSize = 500
	lastBufSize       = 10
)

// StdFacilityName --
const StdFacilityName = ""

// Facility --
type Facility struct {
	name  string
	level Level
}

type sysWriter struct{}

var (
	mutex = new(sync.Mutex)

	levels = []logLevelDef{
		{EMERG, "EMERG", "EM"},
		{ALERT, "ALERT", "AL"},
		{CRIT, "CRIT", "CR"},
		{ERR, "ERR", "ER"},
		{WARNING, "WARNING", "WA"},
		{NOTICE, "NOTICE", "NO"},
		{INFO, "INFO", "IN"},
		{TIME, "TIME", "TM"},
		{DEBUG, "DEBUG", "DE"},
		{TRACE1, "TRACE1", "T1"},
		{TRACE2, "TRACE2", "T2"},
		{TRACE3, "TRACE3", "T3"},
		{TRACE4, "TRACE4", "T4"},
		{UNKNOWN, "UNKNOWN", "??"},
	}

	facilities  = map[string]*Facility{}
	stdFacility *Facility

	consoleWriter io.Writer

	enabled   = true
	active    = true
	firstTime = true

	dumpFileName  = "unsaved.log"
	beforeFileBuf = []string{}

	lastBuf = []string{}

	logFuncName = logFuncNameNone

	localTime     = false
	lastWriteDate string

	fileDirectory   string
	fileNamePattern string
	fileName        string
	file            *os.File

	writer = &sysWriter{}

	fileWriterBufSize     = 0
	fileWriter            *bufio.Writer
	fileWriterMutex       = new(sync.Mutex)
	fileWriterFlushPeriod = 0 * time.Second

	maxLen = 0

	pid int
)

// ChangeLevelAlertFunc --
type ChangeLevelAlertFunc func(facility string, old Level, new Level)

var (
	alertSubscriberID = int64(0)
	alertSubscribers  = map[int64]ChangeLevelAlertFunc{}
)

//----------------------------------------------------------------------------------------------------------------------------//

func init() {
	pid = os.Getpid()
	misc.AddExitFunc("log.exit", exit, nil)

	stdFacility = NewFacility(StdFacilityName)

	consoleWriter = &ConsoleWriter{}

	log.SetFlags(0)
	log.SetOutput(writer)

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

// Writer --
func Writer() io.Writer {
	return writer
}

func (l *sysWriter) Write(p []byte) (int, error) {
	MessageEx(1, NOTICE, nil, "%s", strings.TrimSpace(string(p)))
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
	l.stream.Log(strings.TrimSpace(string(p)))
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
	if fileWriter != nil {
		fileWriterMutex.Lock()
		fileWriter.Flush()
		fileWriterMutex.Unlock()
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
	panicID := panic.ID()
	defer panic.SaveStackToLogEx(panicID)

	var period time.Duration
	lastFlushDate := ""

	for {
		if fileWriterFlushPeriod == 0 {
			period = 1 * time.Second
		} else {
			period = fileWriterFlushPeriod
		}

		if !misc.Sleep(period) {
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
	mutex.Lock()
	defer mutex.Unlock()

	list := make([]string, len(lastBuf))
	for i, s := range lastBuf {
		list[i] = strings.TrimSpace(s)
	}

	return list
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

	for _, def := range levels {
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

	for _, def := range levels {
		if def.code < UNKNOWN {
			list[def.code] = def.name
		}
	}

	return list
}

// GetLogLevelName -- get log level names
func GetLogLevelName(level Level) (short string, long string) {
	return levels[level].shortName, levels[level].name
}

//----------------------------------------------------------------------------------------------------------------------------//

// AddAlertFunc --
func AddAlertFunc(f ChangeLevelAlertFunc) int64 {
	mutex.Lock()
	defer mutex.Unlock()

	alertSubscriberID++
	alertSubscribers[alertSubscriberID] = f
	return alertSubscriberID
}

// DelAlertFunc --
func DelAlertFunc(id int64) {
	mutex.Lock()
	defer mutex.Unlock()

	delete(alertSubscribers, id)
}

//----------------------------------------------------------------------------------------------------------------------------//

// SetFile -- file for log
func SetFile(directory string, suffix string, useLocalTime bool, bufSize int, flushPeriod time.Duration) {
	if directory == "" {
		directory = "./logs/"
	}

	fileDirectory = directory
	localTime = useLocalTime
	fileWriterBufSize = bufSize

	if flushPeriod > 0 {
		fileWriterFlushPeriod = flushPeriod
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
		if fileWriter != nil {
			fileWriterMutex.Lock()
			fileWriter.Write([]byte(s))
			fileWriterMutex.Unlock()
		} else {
			file.Write([]byte(s))
		}
	}
}

func openLogFile(dt string) {

	if file != nil {
		if fileWriter != nil {
			fileWriterMutex.Lock()
			fileWriter.Flush()
			fileWriter = nil
			fileWriterMutex.Unlock()
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

	tags := misc.AppTags()
	if tags != "" {
		tags = " " + tags
	}

	msg := fmt.Sprintf("[%d] %s %s *** %s %s%s%s was launched at %sZ with command line \"%s\"",
		pid,
		levels[INFO].shortName,
		now().Format(misc.DateTimeFormatRevWithMS),
		misc.AppName(),
		misc.AppVersion(),
		tags,
		ts,
		t.Format(misc.DateTimeFormatRev),
		cmd)

	if maxLen > 0 && maxLen < len(msg) {
		msg = msg[:maxLen]
	}
	msg += misc.EOS

	if file != nil {
		if fileWriterBufSize > 0 {
			fileWriter = bufio.NewWriterSize(file, fileWriterBufSize)
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

func logger(withLock bool, stackShift int, facility string, level Level, replace *misc.Replace, message string, params ...interface{}) {
	if !enabled {
		return
	}

	if withLock {
		mutex.Lock()
		defer mutex.Unlock()
	}

	levelName := ""
	if (level >= EMERG) && (level < UNKNOWN) {
		levelName = levels[level].shortName
	} else {
		levelName = fmt.Sprintf("?%d?", level)
	}

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

	if facility != "" {
		facility = " <" + facility + ">"
	}

	format := fmt.Sprintf("[%d] %s %s %s%s%s %s", pid, levelName, dt, tm, facility, funcName, message)
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
func StdLogger(facility string, level string, message string, params ...interface{}) {
	nLevel, _ := Str2Level(level)
	GetFacility(facility).Message(nLevel, message, params...)
}

//----------------------------------------------------------------------------------------------------------------------------//

// CurrentLogLevelOfAll -- get all log levels
func CurrentLogLevelOfAll() (list map[string]Level) {
	mutex.Lock()
	defer mutex.Unlock()

	list = make(map[string]Level)
	for name, f := range facilities {
		list[name] = f.level
	}

	return
}

// CurrentLogLevelNamesOfAll -- get all log levels
func CurrentLogLevelNamesOfAll() (list misc.StringMap) {
	mutex.Lock()
	defer mutex.Unlock()

	list = make(misc.StringMap)
	for name, f := range facilities {
		_, n := GetLogLevelName(f.level)
		list[name] = n
	}

	return
}

// SetLogLevels -- set log level
func SetLogLevels(defaultLevelName string, levels misc.StringMap, logFunc FuncNameMode) (err error) {
	mutex.Lock()
	defer mutex.Unlock()

	for _, f := range facilities {
		level, exists := levels[f.name]
		if !exists {
			level = defaultLevelName
		}
		_, err = f.setLogLevel(level, logFunc)
		if err != nil {
			return
		}
	}

	return
}

//----------------------------------------------------------------------------------------------------------------------------//

// NewFacility --
func NewFacility(name string) *Facility {
	mutex.Lock()
	defer mutex.Unlock()

	f, exists := facilities[name]
	if exists {
		return f
	}

	level := DEBUG
	if name != StdFacilityName {
		level = stdFacility.level
	}

	f = &Facility{
		name:  name,
		level: level,
	}

	facilities[name] = f
	return f
}

// GetFacility --
func GetFacility(name string) *Facility {
	mutex.Lock()
	defer mutex.Unlock()

	f, exists := facilities[name]
	if exists {
		return f
	}

	return NewFacility(name)
}

// Name -- get facility name
func (f *Facility) Name() (name string) {
	return f.name
}

// CurrentLogLevel -- get log level
func (f *Facility) CurrentLogLevel() (level Level) {
	return f.level
}

// CurrentLogLevelEx -- get log level
func (f *Facility) CurrentLogLevelEx() (level Level, short string, long string) {
	level = f.level
	short, long = GetLogLevelName(level)
	return
}

// SetLogLevel -- set log level
func (f *Facility) SetLogLevel(levelName string, funcNameMode FuncNameMode) (oldLevel Level, err error) {
	mutex.Lock()
	defer mutex.Unlock()

	return f.setLogLevel(levelName, funcNameMode)
}

func (f *Facility) setLogLevel(levelName string, funcNameMode FuncNameMode) (oldLevel Level, err error) {
	switch funcNameMode {
	case FuncNameModeShort:
		logFuncName = logFuncNameShort
	case FuncNameModeFull:
		logFuncName = logFuncNameFull
	case FuncNameModeNone:
		fallthrough
	default:
		logFuncName = logFuncNameNone
	}

	oldLevel = f.level

	newLevel, ok := Str2Level(levelName)
	if !ok {
		msg := fmt.Sprintf(`Invalid log level "%s", left unchanged "%s" `, levelName, levels[oldLevel].name)
		err = errors.New(msg)
		logger(false, 0, f.name, WARNING, nil, msg)
		return
	}

	if newLevel != oldLevel {
		for _, alert := range alertSubscribers {
			alert(f.name, oldLevel, newLevel)
		}

		f.level = newLevel
		logger(false, 0, f.name, INFO, nil, `Log level is "%s"`, levels[newLevel].name)
	}

	return
}

// MessageEx -- add message to the log with custom shift
func (f *Facility) MessageEx(shift int, level Level, replace *misc.Replace, message string, params ...interface{}) {
	if level <= f.level {
		if level < 0 {
			level = -level
		}
		logger(true, shift+1, f.name, level, replace, message, params...)
	}
}

// Message -- add message to the log
func (f *Facility) Message(level Level, message string, params ...interface{}) {
	f.MessageEx(1, level, nil, message, params...)
}

// MessageWithSource -- add message to the log with source
func (f *Facility) MessageWithSource(level Level, source string, message string, params ...interface{}) {
	f.MessageEx(1, level, nil, "["+source+"] "+message, params...)
}

// SecuredMessage -- add message to the log with securing
func (f *Facility) SecuredMessage(level Level, replace *misc.Replace, message string, params ...interface{}) {
	f.MessageEx(1, level, replace, message, params...)
}

// SecuredMessageWithSource -- add message to the log with source & securing
func (f *Facility) SecuredMessageWithSource(level Level, replace *misc.Replace, source string, message string, params ...interface{}) {
	f.MessageEx(1, level, replace, "["+source+"] "+message, params...)
}

//----------------------------------------------------------------------------------------------------------------------------//

// StdFacility --
func StdFacility() *Facility {
	return stdFacility
}

// CurrentLogLevel --
func CurrentLogLevel() (level Level) {
	return stdFacility.CurrentLogLevel()
}

// CurrentLogLevelEx --
func CurrentLogLevelEx() (level Level, short string, long string) {
	return stdFacility.CurrentLogLevelEx()
}

// SetLogLevel -- set log level
func SetLogLevel(levelName string, logFunc FuncNameMode) (oldLevel Level, err error) {
	return stdFacility.SetLogLevel(levelName, logFunc)
}

// MessageEx -- add message to the log with custom shift
func MessageEx(shift int, level Level, replace *misc.Replace, message string, params ...interface{}) {
	stdFacility.MessageEx(shift, level, replace, message, params...)
}

// Message -- add message to the log
func Message(level Level, message string, params ...interface{}) {
	stdFacility.Message(level, message, params...)
}

// SecuredMessage -- add message to the log with securing
func SecuredMessage(level Level, replace *misc.Replace, message string, params ...interface{}) {
	stdFacility.SecuredMessage(level, replace, message, params...)
}

// MessageWithSource -- add message to the log with source
func MessageWithSource(level Level, source string, message string, params ...interface{}) {
	stdFacility.MessageWithSource(level, source, message, params...)
}

// SecuredMessageWithSource -- add message to the log with source & securing
func SecuredMessageWithSource(level Level, replace *misc.Replace, source string, message string, params ...interface{}) {
	stdFacility.SecuredMessageWithSource(level, replace, source, message, params...)
}

//----------------------------------------------------------------------------------------------------------------------------//
