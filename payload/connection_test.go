package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:22
 */

func TestConnectionEndpointResponse_Valid(t *testing.T) {
	resp := &ConnectionEndpointResponse{
		Endpoint: "ep",
		Ticket:   "ti",
	}

	assert.Nil(t, resp.Valid())

	resp.Endpoint = ""
	assert.NotNil(t, resp.Valid())

	resp = nil
	assert.NotNil(t, resp.Valid())
}
