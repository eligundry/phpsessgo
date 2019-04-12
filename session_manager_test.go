package phpsessgo_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/imantung/phpsessgo"
	"github.com/imantung/phpsessgo/mock"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_New(t *testing.T) {
	config := phpsessgo.SessionConfig{
		Name: "some-session-name",
	}
	manager, _ := phpsessgo.NewSessionManager(config)
	require.Equal(t, "some-session-name", manager.SessionName)
}

func TestSessionManager_Start_GenerateSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sidCreator := mock.NewMockSessionIDCreator(ctrl)
	sidCreator.EXPECT().CreateSID().Return("random-hash")

	handler := mock.NewMockSessionHandler(ctrl)

	manager := phpsessgo.SessionManager{
		SessionName: "some-session-name",
		SIDCreator:  sidCreator,
		Handler:     handler,
	}

	req, _ := http.NewRequest(http.MethodGet, "some-url", nil)
	rr := httptest.NewRecorder()

	session, err := manager.Start(rr, req)
	require.NoError(t, err)
	require.Equal(t, "random-hash", session.SessionID)
	require.Equal(t, "some-session-name=random-hash", rr.HeaderMap.Get("Set-Cookie"))
}

func TestSessionManager_Start_ExistingSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sidCreator := mock.NewMockSessionIDCreator(ctrl)
	handler := mock.NewMockSessionHandler(ctrl)

	manager := phpsessgo.SessionManager{
		SessionName: "some-session-name",
		SIDCreator:  sidCreator,
		Handler:     handler,
	}

	req, _ := http.NewRequest(http.MethodGet, "some-url", nil)
	req.AddCookie(&http.Cookie{
		Name:  "some-session-name",
		Value: "some-session-id",
	})

	t.Run("handler failed to read", func(t *testing.T) {
		handler.EXPECT().Read("some-session-id").Return("", fmt.Errorf("some-error"))

		_, err := manager.Start(nil, req)
		require.EqualError(t, err, "some-error")
	})

	t.Run("name", func(t *testing.T) {
		handler.EXPECT().Read("some-session-id").Return("some-data", nil)

		session, err := manager.Start(nil, req)
		require.NoError(t, err)
		require.Equal(t, "some-session-id", session.SessionID)
	})

}
