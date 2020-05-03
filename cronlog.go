package log

import (
	"fmt"
	"time"

	"github.com/alrusov/bufpool"
	"github.com/alrusov/misc"
)

//----------------------------------------------------------------------------------------------------------------------------//

// CronLog --
type CronLog struct{}

//----------------------------------------------------------------------------------------------------------------------------//

// Info --
func (cl *CronLog) Info(msg string, keysAndValues ...interface{}) {
	Message(TRACE2, cl.makeMsg(nil, msg, keysAndValues...))
}

// Error --
func (cl *CronLog) Error(err error, msg string, keysAndValues ...interface{}) {
	Message(ERR, cl.makeMsg(err, msg, keysAndValues...))
}

//----------------------------------------------------------------------------------------------------------------------------//

func (cl *CronLog) makeMsg(err error, msg string, keysAndValues ...interface{}) string {
	var out = bufpool.GetBuf()
	defer bufpool.PutBuf(out)

	if err != nil {
		out.WriteString(err.Error())
	}

	if msg != "" {
		if out.Len() != 0 {
			out.WriteString(": ")
		}
		out.WriteString(msg)
	}

	kvFmt := cl.makeFmt(keysAndValues...)
	if kvFmt != "" {
		if out.Len() == 0 {
			out.WriteString(kvFmt)
		} else {
			out.WriteString(" (" + kvFmt + ")")
		}
	}

	return fmt.Sprintf("[cron] "+string(out.Bytes()), keysAndValues...)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (cl *CronLog) makeFmt(p ...interface{}) string {
	n := len(p)

	if n == 0 {
		return ""
	}

	var fmt = bufpool.GetBuf()
	defer bufpool.PutBuf(fmt)

	for i := 0; i < n; i += 2 {
		if i > 0 {
			fmt.WriteString(", ")
		}
		fmt.WriteString("%v=%v")

		i2 := i + 1
		switch p[i2].(type) {
		case time.Time:
			p[i2] = p[i2].(time.Time).Format(misc.DateTimeFormatJSON)
		}
	}

	return fmt.String()
}

//----------------------------------------------------------------------------------------------------------------------------//
