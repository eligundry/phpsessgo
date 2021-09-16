package phpsessgo_test

import (
	"reflect"
	"testing"

	"github.com/eligundry/phpsessgo"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/require"
)

func TestNewRedisSessionManager(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	manager := phpsessgo.NewRedisSessionManager(client, phpsessgo.SessionManagerConfig{})
	require.Equal(t, phpsessgo.DefaultSessionName, manager.SessionName())
	require.Equal(t, "*phpsessgo.UUIDCreator", reflect.TypeOf(manager.SIDCreator()).String())
	require.Equal(t, "*phpsessgo.PHPSessionEncoder", reflect.TypeOf(manager.Encoder()).String())
	require.Equal(t, "*phpsessgo.RedisSessionHandler", reflect.TypeOf(manager.Handler()).String())
}
