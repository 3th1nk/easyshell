package telnet

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	client *Client
)

func TestMain(m *testing.M) {
	var err error
	client, err = NewClient(&ClientConfig{
		Addr:     "192.168.2.14:23",
		User:     "admin",
		Password: "geesunn123",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	m.Run()
}

func TestClient_Read(t *testing.T) {
	defer func() {
		assert.NoError(t, client.Close())
	}()
	t.Log(client.welcomeStr)
	t.Log(client.promptStr)

	cmd := "show version"
	_, err := client.Write([]byte(cmd + "\n"))
	assert.NoError(t, err)

	data, _, err := client.ReadUtil2(cmd)
	assert.NoError(t, err)
	t.Log(data.String())

	data, _, err = client.doReadUtilPrompt()
	assert.NoError(t, err)
	t.Log(data.String())
}
