package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFirstLanIP(t *testing.T) {
	ip, err := GetFirstLanIP()
	assert.Nil(t, err)
	assert.NotEmpty(t, ip)
}
