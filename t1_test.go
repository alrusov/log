package log

import (
	"errors"
	"testing"
	"time"
)

//----------------------------------------------------------------------------------------------------------------------------//

func TestCronlog(t *testing.T) {
	type paramsBlock struct {
		err    error
		msg    string
		kv     []interface{}
		expect string
	}

	err := errors.New("Something went wrong")
	msg := "Test message"

	params := []paramsBlock{
		{nil, "", []interface{}{}, "[cron] "},
		{nil, "", []interface{}{"p1", "v1", "p2", 3, "p3", time.Unix(60, 123456789).UTC()}, "[cron] p1=v1, p2=3, p3=1970-01-01T00:01:00.123Z"},
		{nil, msg, []interface{}{}, "[cron] Test message"},
		{nil, msg, []interface{}{"p1", "v1", "p2", 3, "p3", time.Unix(60, 123456789).UTC()}, "[cron] Test message (p1=v1, p2=3, p3=1970-01-01T00:01:00.123Z)"},
		{err, "", []interface{}{}, "[cron] Something went wrong"},
		{err, "", []interface{}{"p1", "v1", "p2", 3, "p3", time.Unix(60, 123456789).UTC()}, "[cron] Something went wrong (p1=v1, p2=3, p3=1970-01-01T00:01:00.123Z)"},
		{err, msg, []interface{}{}, "[cron] Something went wrong: Test message"},
		{err, msg, []interface{}{"p1", "v1", "p2", 3, "p3", time.Unix(60, 123456789).UTC()}, "[cron] Something went wrong: Test message (p1=v1, p2=3, p3=1970-01-01T00:01:00.123Z)"},
	}

	cl := &CronLog{}

	for i, p := range params {
		i++
		m := cl.makeMsg(p.err, p.msg, p.kv...)
		if m != p.expect {
			t.Errorf(`%d: message "%s", expected "%s"`, i, m, p.expect)
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------------//
