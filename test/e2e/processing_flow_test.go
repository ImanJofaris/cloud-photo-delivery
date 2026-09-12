//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/imaging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
)

func jpegBytesE2E(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 64, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func TestE2E_UploadProcessingFlow(t *testing.T) {
	h, pool, endpoint := setupUploadsAPIWithEndpoint(t)

	// Sign up, create an event.
	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"proc@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Processing Event"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)

	body := jpegBytesE2E(t, 1200, 800)

	// Initialize a simple upload.
	initBody, _ := json.Marshal(map[string]any{
		"filename": "guest.jpg", "contentType": "image/jpeg", "size": len(body),
	})
	rec = doReqHeaders(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/uploads",
		string(initBody), access, map[string]string{"Idempotency-Key": "proc-1"})
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var initEnv struct {
		Data struct {
			PhotoID   string `json:"photoId"`
			UploadURL string `json:"uploadUrl"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &initEnv))

	// PUT the real JPEG to MinIO.
	req, err := http.NewRequest(http.MethodPut, initEnv.Data.UploadURL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Complete -> PROCESSING with a job enqueued.
	rec = doReq(t, h, http.MethodPost, "/api/v1/uploads/"+initEnv.Data.PhotoID+"/complete", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "PROCESSING")

	// Run the worker over the real queue/MinIO until the photo is READY.
	ctx := context.Background()
	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
	})
	require.NoError(t, err)

	photoRepo := photos.NewRepository(pool)
	std := imaging.New()
	proc := photos.NewProcessor(photoRepo, store, std, std, std, slog.Default(), photoRepo.EventOwner)
	registry := jobs.NewRegistry().Register(uploadsJobType, proc.Handle)
	queue := jobs.NewPostgresQueue(pool)
	poller := jobs.NewPoller(queue, registry, slog.Default(), "e2e-worker",
		jobs.WithConcurrency(1), jobs.WithPollInterval(20*time.Millisecond))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { poller.Run(runCtx); close(done) }()

	require.Eventually(t, func() bool {
		var status string
		err := pool.QueryRow(ctx, `SELECT status FROM photos WHERE id = $1`, initEnv.Data.PhotoID).Scan(&status)
		return err == nil && status == "READY"
	}, 30*time.Second, 100*time.Millisecond, "photo should become READY")

	// Status endpoint reflects READY with a thumbnail key present.
	rec = doReq(t, h, http.MethodGet, "/api/v1/uploads/"+initEnv.Data.PhotoID, "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "READY")

	var thumbnailKey string
	err = pool.QueryRow(ctx, `SELECT thumbnail_key FROM photos WHERE id = $1`, initEnv.Data.PhotoID).Scan(&thumbnailKey)
	require.NoError(t, err)
	require.NotEmpty(t, thumbnailKey)

	cancel()
	<-done
}

const uploadsJobType = "PROCESS_PHOTO"
