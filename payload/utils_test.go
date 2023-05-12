package payload

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/4/7 15:13
 */

func TestGenerateMessageId(t *testing.T) {
	assert.NotEqual(t, "", GenerateMessageId("prefix-"))
	assert.True(t, strings.HasPrefix(GenerateMessageId("prefix-"), "prefix-"))
}
