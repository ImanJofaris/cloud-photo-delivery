//go:build integration

package r2_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startMinio(t *testing.T) (endpoint string) {
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

	return "http://" + host + ":" + port.Port()
}

// createBucket uses the AWS SDK from the test process (which can reach the
// mapped port on the host) instead of an mc sidecar container.
func createBucket(t *testing.T, endpoint, bucket string) {
	t.Helper()
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	require.NoError(t, err)

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
}

func TestS3Store_RoundTrip(t *testing.T) {
	ctx := context.Background()
	endpoint := startMinio(t)
	createBucket(t, endpoint, "cpd-photos")

	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
		UseSSL:    false,
	})
	require.NoError(t, err)
	require.Equal(t, "cpd-photos", store.Bucket())

	body := []byte("hello world")
	require.NoError(t, store.Put(ctx, "test/hello.txt", "text/plain", body))

	size, err := store.Head(ctx, "test/hello.txt")
	require.NoError(t, err)
	require.EqualValues(t, len(body), size)

	got, err := store.Get(ctx, "test/hello.txt")
	require.NoError(t, err)
	require.Equal(t, body, got)

	url, err := store.PresignGet(ctx, "test/hello.txt", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, url)

	putURL, err := store.PresignPut(ctx, "test/new.txt", "text/plain", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, putURL)

	require.NoError(t, store.Delete(ctx, "test/hello.txt"))

	_, err = store.Head(ctx, "test/hello.txt")
	require.ErrorIs(t, err, r2.ErrNotFound)
}

func TestS3Store_MultipartUpload(t *testing.T) {
	ctx := context.Background()
	endpoint := startMinio(t)
	createBucket(t, endpoint, "cpd-photos")

	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
		UseSSL:    false,
	})
	require.NoError(t, err)

	key := "test/multipart.bin"
	uploadID, err := store.CreateMultipartUpload(ctx, key, "application/octet-stream")
	require.NoError(t, err)
	require.NotEmpty(t, uploadID)

	partBody := bytes.Repeat([]byte("a"), 5*1024*1024)
	partURL, err := store.PresignUploadPart(ctx, key, uploadID, 1, time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, partURL)

	req, err := http.NewRequest(http.MethodPut, partURL, bytes.NewReader(partBody))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)

	require.NoError(t, store.CompleteMultipartUpload(ctx, key, uploadID, []r2.CompletePart{
		{PartNumber: 1, ETag: etag},
	}))

	size, err := store.Head(ctx, key)
	require.NoError(t, err)
	require.EqualValues(t, len(partBody), size)

	abortID, err := store.CreateMultipartUpload(ctx, "test/abort.bin", "application/octet-stream")
	require.NoError(t, err)
	require.NoError(t, store.AbortMultipartUpload(ctx, "test/abort.bin", abortID))
}
