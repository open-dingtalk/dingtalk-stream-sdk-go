package logger

import (
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:32
 */

func TestStdLogger_Output(t *testing.T) {
	stdLogger := NewStdTestLogger()

	stdLogger.Debugf("logger level: %s", "debug")
	stdLogger.Infof("logger level: %s", "info")
	stdLogger.Warningf("logger level: %s", "warning")
	stdLogger.Errorf("logger level: %s", "error")
	stdLogger.Fatalf("logger level: %s", "fatal")
}
