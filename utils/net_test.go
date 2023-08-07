package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetFirstLanIP(t *testing.T) {
	ip, err := GetFirstLanIP()
	assert.Nil(t, err)
	assert.NotEmpty(t, ip)
}
