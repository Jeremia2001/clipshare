package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type RustFSClient struct {
	client  *minio.Client
	buckets struct {
		Clips      string
		Thumbnails string
	}
	endpoint       string
	publicEndpoint string
	useSSL         bool
}

func NewRustFSClient(endpoint, accessKey, secretKey string, useSSL bool, publicEndpoint string, bucketConfig struct {
	Clips      string
	Thumbnails string
}) (*RustFSClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create RustFS client: %w", err)
	}

	r := &RustFSClient{
		client:         client,
		endpoint:       endpoint,
		publicEndpoint: publicEndpoint,
		useSSL:         useSSL,
	}
	r.buckets.Clips = bucketConfig.Clips
	r.buckets.Thumbnails = bucketConfig.Thumbnails

	ctx := context.Background()
	for _, bucket := range []string{r.buckets.Clips, r.buckets.Thumbnails} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to check bucket %s: %w", bucket, err)
		}
		if !exists {
			err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
	}

	return r, nil
}

func (r *RustFSClient) GeneratePresignedViewURL(ctx context.Context, bucket, objectKey string, expires time.Duration) (*url.URL, error) {
	// Generate presigned URL using the internal client
	u, err := r.client.PresignedGetObject(ctx, bucket, objectKey, expires, nil)
	if err != nil {
		return nil, err
	}

	// Rewrite host to public endpoint so external clients can reach it
	if r.publicEndpoint != "" {
		pub, err := url.Parse(r.publicEndpoint)
		if err == nil {
			u.Scheme = pub.Scheme
			u.Host = pub.Host
		}
	}

	return u, nil
}

func (r *RustFSClient) DeleteObject(ctx context.Context, bucket, objectKey string) error {
	return r.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

func (r *RustFSClient) PutObject(ctx context.Context, bucket, objectKey string, reader io.Reader, objectSize int64, contentType string) error {
	_, err := r.client.PutObject(ctx, bucket, objectKey, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (r *RustFSClient) StatObject(ctx context.Context, bucket, objectKey string) (minio.ObjectInfo, error) {
	return r.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
}

func (r *RustFSClient) GetObject(ctx context.Context, bucket, objectKey string) (*minio.Object, error) {
	return r.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
}

// GetObjectRange fetches only the bytes [start, end] (both inclusive) from the
// object store, avoiding a full-file download when serving HTTP range requests.
//
// Workaround for a rustfs bug: ranges that begin anywhere in the second half
// of an internal 16 MiB block come back as 0 bytes + "unexpected EOF". We
// round `start` down to the nearest 16 MiB boundary when requesting from
// rustfs, then skip the extra prefix bytes so the caller sees a reader that
// begins at the real `start`. Worst case this fetches ~16 MiB of unused data
// (intra-host to rustfs), but only the client-requested window is streamed on
// the wire.
func (r *RustFSClient) GetObjectRange(ctx context.Context, bucket, objectKey string, start, end int64) (io.ReadCloser, error) {
	const alignment int64 = 16 * 1024 * 1024
	alignedStart := start - (start % alignment)

	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(alignedStart, end); err != nil {
		return nil, err
	}
	obj, err := r.client.GetObject(ctx, bucket, objectKey, opts)
	if err != nil {
		return nil, err
	}

	if skip := start - alignedStart; skip > 0 {
		if _, err := io.CopyN(io.Discard, obj, skip); err != nil {
			obj.Close()
			return nil, err
		}
	}
	return obj, nil
}

func (r *RustFSClient) BucketNames() (clips, thumbnails string) {
	return r.buckets.Clips, r.buckets.Thumbnails
}
