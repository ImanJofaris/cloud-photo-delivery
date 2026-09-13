//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type deviceEnvelope struct {
	Data struct {
		Device struct {
			ID              string  `json:"id"`
			Name            string  `json:"name"`
			KeyPrefix       string  `json:"keyPrefix"`
			AssignedEventID *string `json:"assignedEventId"`
		} `json:"device"`
		Key string `json:"key"`
	} `json:"data"`
}

func createDevice(t *testing.T, h http.Handler, access, name, eventID string) deviceEnvelope {
	t.Helper()
	rec := doReq(t, h, http.MethodPost, "/api/v1/devices",
		`{"name":"`+name+`","assignedEventId":"`+eventID+`"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	var env deviceEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotEmpty(t, env.Data.Key)
	require.Equal(t, name, env.Data.Device.Name)
	require.NotNil(t, env.Data.Device.AssignedEventID)
	require.Equal(t, eventID, *env.Data.Device.AssignedEventID)
	return env
}

func deviceInit(t *testing.T, h http.Handler, key, eventID string) *httptest.ResponseRecorder {
	t.Helper()
	return doReqHeaders(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/uploads",
		`{"filename":"booth.jpg","contentType":"image/jpeg","size":12}`, "",
		map[string]string{"X-Api-Key": key})
}

func TestE2E_DeviceUploadScopeAndRotation(t *testing.T) {
	h, pool := setupUploadsAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"ops@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Assigned Event"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventA := extractEventID(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Other Event"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventB := extractEventID(t, rec)

	device := createDevice(t, h, access, "Booth 1", eventA)

	// Operator-only tenant isolation on the device list.
	recA := doReq(t, h, http.MethodPost, "/api/v1/auth/signup", `{"email":"b@example.com","password":"password123"}`, "")
	accessB, _ := parseAuth(t, recA)
	rec = doReq(t, h, http.MethodGet, "/api/v1/devices", "", accessB)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), device.Data.Device.ID)

	// Device initializes and completes an upload for its assigned event.
	initRec := deviceInit(t, h, device.Data.Key, eventA)
	require.Equal(t, http.StatusCreated, initRec.Code, "body=%s", initRec.Body)
	var initEnv struct {
		Data struct {
			PhotoID   string `json:"photoId"`
			UploadURL string `json:"uploadUrl"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(initRec.Body.Bytes(), &initEnv))
	require.NotEmpty(t, initEnv.Data.UploadURL)

	body := []byte("hello world!")
	req, err := http.NewRequest(http.MethodPut, initEnv.Data.UploadURL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	rec = doReqHeaders(t, h, http.MethodPost, "/api/v1/uploads/"+initEnv.Data.PhotoID+"/complete", "",
		"", map[string]string{"X-Api-Key": device.Data.Key})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "PROCESSING")

	// Device cannot initialize uploads for a foreign event.
	rec = deviceInit(t, h, device.Data.Key, eventB)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")

	// Operator JWT still works on the same endpoint.
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventB+"/uploads",
		`{"filename":"op.jpg","contentType":"image/jpeg","size":12}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	// Rotation invalidates the old key immediately.
	rec = doReq(t, h, http.MethodPost, "/api/v1/devices/"+device.Data.Device.ID+"/rotate", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var rotated deviceEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rotated))
	require.NotEqual(t, device.Data.Key, rotated.Data.Key)

	rec = deviceInit(t, h, device.Data.Key, eventA)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_DEVICE_KEY")

	rec = deviceInit(t, h, rotated.Data.Key, eventA)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	// Revocation stops the rotated key too.
	rec = doReq(t, h, http.MethodDelete, "/api/v1/devices/"+device.Data.Device.ID, "", access)
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = deviceInit(t, h, rotated.Data.Key, eventA)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "DEVICE_REVOKED")

	// last_used_at was recorded on the successful device calls.
	var lastUsed *string
	err = pool.QueryRow(context.Background(),
		`SELECT last_used_at::text FROM devices WHERE id = $1`, device.Data.Device.ID).Scan(&lastUsed)
	require.NoError(t, err)
	require.NotNil(t, lastUsed)
}
