// Copyright 2026 The Ratel Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package storage

import (
	"context"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cockroachdb/errors"
)

// ErrBucketNotFound indicates the storage bucket does not exist.
var ErrBucketNotFound = errors.New("bucket not found")

// ProbeStorage checks whether the storage at the given URL is accessible.
// Returns ErrBucketNotFound if the bucket does not exist. For file:// URLs
// this always succeeds because directories are created on demand.
func ProbeStorage(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "file":
		return nil
	case "s3":
		cfg := parseS3URL(u)
		return probeS3Bucket(ctx, cfg)
	case "http", "https":
		cfg := parseHTTPAsS3(u)
		return probeS3Bucket(ctx, cfg)
	default:
		return nil
	}
}

// CreateBucket creates the bucket for the given URL. For file:// URLs this is
// a no-op.
func CreateBucket(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "file":
		return nil
	case "s3":
		cfg := parseS3URL(u)
		return createS3Bucket(ctx, cfg)
	case "http", "https":
		cfg := parseHTTPAsS3(u)
		return createS3Bucket(ctx, cfg)
	default:
		return errors.Errorf("bucket creation not supported for scheme %q", u.Scheme)
	}
}

// BucketName extracts the bucket name from a storage URL.
func BucketName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	switch u.Scheme {
	case "file":
		return u.Path
	case "s3":
		return u.Host
	case "http", "https":
		path := strings.TrimPrefix(u.Path, "/")
		bucket, _, _ := strings.Cut(path, "/")
		return bucket
	default:
		return rawURL
	}
}

func parseHTTPAsS3(u *url.URL) S3StorageConfig {
	path := strings.TrimPrefix(u.Path, "/")
	bucket, prefix, _ := strings.Cut(path, "/")
	region := u.Query().Get("region")
	if region == "" {
		region = "us-east-1"
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return S3StorageConfig{
		Bucket:           bucket,
		Prefix:           prefix,
		Region:           region,
		Endpoint:         u.Scheme + "://" + u.Host,
		S3ForcePathStyle: true,
	}
}

func probeS3Bucket(ctx context.Context, cfg S3StorageConfig) error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.Region),
		Endpoint:         endpointOrNil(cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(cfg.S3ForcePathStyle),
	})
	if err != nil {
		return errors.Wrap(err, "creating AWS session")
	}
	client := s3.New(sess)
	_, err = client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) {
			switch awsErr.Code() {
			case s3.ErrCodeNoSuchBucket, "NotFound":
				return ErrBucketNotFound
			}
		}
		return errors.Wrap(err, "probing bucket")
	}
	return nil
}

func createS3Bucket(ctx context.Context, cfg S3StorageConfig) error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.Region),
		Endpoint:         endpointOrNil(cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(cfg.S3ForcePathStyle),
	})
	if err != nil {
		return errors.Wrap(err, "creating AWS session")
	}
	client := s3.New(sess)
	_, err = client.CreateBucketWithContext(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return errors.Wrapf(err, "creating bucket %q", cfg.Bucket)
	}
	return nil
}
