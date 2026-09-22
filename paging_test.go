package easyshell

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPagingDisableCommand(t *testing.T) {
	assert.Equal(t, "screen-length disable", PagingDisableCommand(VendorH3C))
	assert.Equal(t, "terminal length 0", PagingDisableCommand(VendorCiscoIOS))
	assert.Equal(t, "screen-length 0 temporary", PagingDisableCommand(VendorHuawei))
	assert.Equal(t, "set cli screen-length 0", PagingDisableCommand(VendorJuniper))
	// 未知厂商返回空串(依赖 More 拦截器兜底)
	assert.Equal(t, "", PagingDisableCommand(VendorGeneric))
	assert.Equal(t, "", PagingDisableCommand("unknown-vendor"))
}
