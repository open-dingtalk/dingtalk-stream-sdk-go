package utils

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * @Author linya.jj
 * @Date 2023/3/31 09:51
 */

func TestErrorFromHttpResponse(t *testing.T) {
	assert.NotNil(t, ErrorFromHttpResponseBody(nil))

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("error")),
	}
	assert.NotNil(t, ErrorFromHttpResponseBody(resp))
}
