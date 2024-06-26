package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDefaultPromptRegex(t *testing.T) {
	var rw ReadWriter
	for _, obj := range []struct {
		Prompt string
		Expect bool
	}{
		{"root@test-01 $", true},
		{"root@test-01 #", true},
		{"root@test-01~ $", true},
		{"root@test-01~ # ", true},
		{"root@test-01(active)>", true},
		{"root@test-01(active)> ", true},
		{"root@test-01(M)]", true},
		{"root@test-01(S)] ", true},
		{"root@test-01(active) ] ", true},
		{"root@test-01(active)% ", true},
		{"[root@test-01 ~]#", true},
		{"<A10-8&A10-7_CSZW-Core-Switch>", true},
		{"S-DGB1-H17-WZJR-~(M)# ", true},
		{"S-DGB1-H17-WZJR-~(B)# ", true},
		{"中文名称 #", true},
		{"(CN-SZ-MC01) *#", true},
		{"mtk54007@(szimhM)(cfg-sync Standalone)(Active)(/Common)(tmos)#", true},
		{"root@test-01(active)a ", false},
		{"]", false},
		{"#", false},
		{"$", false},
		{" # ", false},
		{"[mon@m41205302.cloud.208.am49 /home/mon]", true},
		{"[testuser@localhost ~]$ Login:", false},
		{"[testuser@localhost ~]$ Username:", false},
		{"[testuser@localhost ~]$ Password:", false},
	} {
		if obj.Expect != rw.IsEndLine(obj.Prompt) {
			t.Error(obj.Prompt)
		}
	}
}

func TestFindHostname(t *testing.T) {
	for _, obj := range []struct {
		Remaining string
		Hostname  string
	}{
		{"root@HA-备 #", "HA-备"},
		{"[root@localhost ~]#", "localhost"},
		{"[root@localhost.localdomain ~]$", "localhost.localdomain"},
		{"hostname#", "hostname"},
		{"<HUAWEI>hrp enable", "HUAWEI"},
		{"中文主机名 #", "中文主机名"},
		{"HRP_M[HUAWEI] diagnose", "HUAWEI"},
		{"S-ABC-D1-EFG-~(M)# ", "S-ABC-D1-EFG-"},
		{"[mon@m41205302.cloud.208.am49 /home/mon]", "m41205302.cloud.208.am49"},
	} {
		assert.Equal(t, obj.Hostname, findHostname(obj.Remaining))
	}
}
