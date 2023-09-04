package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:50
 */

func TestAppCredentialConfig_Valid(t *testing.T) {
	conf := NewAppCredentialConfig("clientId", "clientSecret")
	assert.Nil(t, conf.Valid())

	conf.ClientId = ""
	assert.NotNil(t, conf.Valid())

	conf = nil
	assert.NotNil(t, conf.Valid())
}

func TestDingtalkGoSDKUserAgent_Valid(t *testing.T) {
	conf := NewDingtalkGoSDKUserAgent()
	assert.Nil(t, conf.Valid())

	conf.UserAgent = ""
	assert.NotNil(t, conf.Valid())

	conf = nil
	assert.NotNil(t, conf.Valid())
}
