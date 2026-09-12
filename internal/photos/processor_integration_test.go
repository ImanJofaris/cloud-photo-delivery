//go:build integration

package photos_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/imaging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startMinio(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "9000")
	require.NoError(t, err)
	endpoint := "http://" + host + ":" + port.Port()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	require.NoError(t, err)
	client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String("cpd-photos")})
	require.NoError(t, err)
	return endpoint
}

func newS3Store(t *testing.T, endpoint string) *r2.S3Store {
	t.Helper()
	store, err := r2.New(context.Background(), r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
	})
	require.NoError(t, err)
	return store
}

func jpegFixture(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 128, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func TestProcessor_PipelineAgainstMinioAndPostgres(t *testing.T) {
	pool, userA, _ := setupDB(t)
	endpoint := startMinio(t)
	store := newS3Store(t, endpoint)
	ctx := context.Background()

	eventID := insertEvent(t, pool, userA, "pipeline")
	photo := createUploadingPhoto(t, pool, eventID, 0)

	// Put the real original into MinIO at the photo's storage key.
	original := jpegFixture(t, 3000, 2000)
	require.NoError(t, store.Put(ctx, photo.StorageKey, "image/jpeg", original))

	repo := photos.NewRepository(pool)
	_, _, err := repo.MarkProcessing(ctx, photo.ID, int64(len(original)))
	require.NoError(t, err)

	std := imaging.New()
	proc := photos.NewProcessor(repo, store, std, std, std, slog.Default(), repo.EventOwner)
	payload, _ := json.Marshal(photos.ProcessPayload{PhotoID: photo.ID, EventID: eventID})
	require.NoError(t, proc.Handle(ctx, payload))

	got, err := repo.GetByID(ctx, photo.ID)
	require.NoError(t, err)
	require.Equal(t, photos.StatusReady, got.Status)
	require.NotNil(t, got.Width)
	require.NotNil(t, got.Height)
	require.Equal(t, 3000, *got.Width)
	require.Equal(t, 2000, *got.Height)
	require.NotNil(t, got.ThumbnailKey)
	require.NotNil(t, got.MediumKey)
	require.NotNil(t, got.OptimizedKey)

	for _, key := range []string{*got.ThumbnailKey, *got.MediumKey, *got.OptimizedKey} {
		size, err := store.Head(ctx, key)
		require.NoError(t, err, "derivative %s should exist", key)
		require.Greater(t, size, int64(0))
	}

	// Idempotency: running again produces no error and no change.
	require.NoError(t, proc.Handle(ctx, payload))
}

func TestProcessor_FailurePathMarksPhotoFailed(t *testing.T) {
	pool, userA, _ := setupDB(t)
	endpoint := startMinio(t)
	store := newS3Store(t, endpoint)
	ctx := context.Background()

	eventID := insertEvent(t, pool, userA, "corrupt")
	photo := createUploadingPhoto(t, pool, eventID, 0)
	require.NoError(t, store.Put(ctx, photo.StorageKey, "image/jpeg", []byte("not an image")))

	repo := photos.NewRepository(pool)
	_, _, err := repo.MarkProcessing(ctx, photo.ID, 12)
	require.NoError(t, err)

	queue := jobs.NewPostgresQueue(pool)
	_, err = queue.Enqueue(ctx, "PROCESS_PHOTO", photos.ProcessPayload{PhotoID: photo.ID, EventID: eventID})
	require.NoError(t, err)

	std := imaging.New()
	proc := photos.NewProcessor(repo, store, std, std, std, slog.Default(), repo.EventOwner)
	registry := jobs.NewRegistry().Register("PROCESS_PHOTO", func(c context.Context, p []byte) error {
		if err := proc.Handle(c, p); err != nil {
			require.NoError(t, repo.MarkFailed(c, photo.ID, "processing failed"))
			return err
		}
		return nil
	})

	poller := jobs.NewPoller(queue, registry, slog.Default(), "test-worker",
		jobs.WithConcurrency(1), jobs.WithPollInterval(10*time.Millisecond))
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { poller.Run(runCtx); close(done) }()

	require.Eventually(t, func() bool {
		p, err := repo.GetByID(ctx, photo.ID)
		return err == nil && p.Status == photos.StatusFailed
	}, 10*time.Second, 50*time.Millisecond)

	var status string
	var attempts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, attempts FROM jobs LIMIT 1`).Scan(&status, &attempts))
	require.Len(t, registry.Types(), 1)
	// After the first failure the job is retried, not yet exhausted.
	require.Equal(t, "pending", status)
	require.Equal(t, 1, attempts)
	cancel()
	<-done
}

func TestProcessor_ConcurrentWorkersProcessEachJobOnce(t *testing.T) {
	pool, userA, _ := setupDB(t)
	endpoint := startMinio(t)
	store := newS3Store(t, endpoint)
	ctx := context.Background()

	eventID := insertEvent(t, pool, userA, "concurrent")
	repo := photos.NewRepository(pool)
	queue := jobs.NewPostgresQueue(pool)

	const n = 5
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		p := createUploadingPhoto(t, pool, eventID, 0)
		require.NoError(t, store.Put(ctx, p.StorageKey, "image/jpeg", jpegFixture(t, 600, 400)))
		_, _, err := repo.MarkProcessing(ctx, p.ID, 100)
		require.NoError(t, err)
		_, err = queue.Enqueue(ctx, "PROCESS_PHOTO", photos.ProcessPayload{PhotoID: p.ID, EventID: eventID})
		require.NoError(t, err)
		ids = append(ids, p.ID)
	}

	std := imaging.New()
	proc := photos.NewProcessor(repo, store, std, std, std, slog.Default(), repo.EventOwner)
	registry := jobs.NewRegistry().Register("PROCESS_PHOTO", proc.Handle)

	runCtx, cancel := context.WithCancel(ctx)
	poller := jobs.NewPoller(queue, registry, slog.Default(), "w",
		jobs.WithConcurrency(4), jobs.WithPollInterval(10*time.Millisecond))
	done := make(chan struct{})
	go func() { poller.Run(runCtx); close(done) }()

	require.Eventually(t, func() bool {
		var ready int
		_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM photos WHERE status = 'READY'`).Scan(&ready)
		return ready == n
	}, 30*time.Second, 100*time.Millisecond)

	var doneJobs int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE status = 'done'`).Scan(&doneJobs))
	require.Equal(t, n, doneJobs)
	cancel()
	<-done
}
