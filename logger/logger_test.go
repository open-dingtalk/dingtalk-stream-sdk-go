package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:30
 */

func TestSetGetSDKLogger(t *testing.T) {
	assert.NotNil(t, GetLogger())

	stdLogger := NewStdTestLogger()
	SetLogger(stdLogger)
	assert.Equal(t, stdLogger, GetLogger())
}
