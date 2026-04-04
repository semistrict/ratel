// Copyright 2024 The Cockroach Authors.
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
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

// S3StorageConfig holds configuration for S3-backed remote storage.
type S3StorageConfig struct {
	// Bucket is the S3 bucket name.
	Bucket string
	// Prefix is prepended to all object names. Should include a trailing slash
	// if non-empty.
	Prefix string
	// Region is the AWS region (e.g. "us-east-1").
	Region string
	// Endpoint overrides the S3 endpoint URL (for MinIO, LocalStack, etc.).
	Endpoint string
	// S3ForcePathStyle forces path-style addressing (required for MinIO).
	S3ForcePathStyle bool
}

// S3StorageFactory implements remote.StorageFactory, creating S3Storage
// instances from locators.
type S3StorageFactory struct {
	config S3StorageConfig
}

var _ remote.StorageFactory = (*S3StorageFactory)(nil)

// NewS3StorageFactory creates a new S3StorageFactory with the given config.
func NewS3StorageFactory(config S3StorageConfig) *S3StorageFactory {
	return &S3StorageFactory{config: config}
}

// CreateStorage implements remote.StorageFactory.
func (f *S3StorageFactory) CreateStorage(locator remote.Locator) (remote.Storage, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(f.config.Region),
		Endpoint:         endpointOrNil(f.config.Endpoint),
		S3ForcePathStyle: aws.Bool(f.config.S3ForcePathStyle),
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating AWS session")
	}
	return &S3Storage{
		bucket:   f.config.Bucket,
		prefix:   f.config.Prefix,
		client:   s3.New(sess),
		uploader: s3manager.NewUploader(sess),
	}, nil
}

func endpointOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// S3Storage implements remote.Storage backed by Amazon S3.
type S3Storage struct {
	bucket   string
	prefix   string
	client   *s3.S3
	uploader *s3manager.Uploader
}

var _ remote.Storage = (*S3Storage)(nil)

func (s *S3Storage) objKey(objName string) string {
	return s.prefix + objName
}

// ReadObject implements remote.Storage.
func (s *S3Storage) ReadObject(
	ctx context.Context, objName string,
) (_ remote.ObjectReader, objSize int64, _ error) {
	headOut, err := s.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.objKey(objName)),
	})
	if err != nil {
		return nil, 0, errors.Wrapf(err, "head object %q", objName)
	}
	size := aws.Int64Value(headOut.ContentLength)
	return &s3ObjectReader{
		s:       s,
		objName: objName,
	}, size, nil
}

// CreateObject implements remote.Storage.
func (s *S3Storage) CreateObject(objName string) (io.WriteCloser, error) {
	return &s3ObjectWriter{
		s:       s,
		objName: objName,
	}, nil
}

// List implements remote.Storage.
func (s *S3Storage) List(prefix, delimiter string) ([]string, error) {
	fullPrefix := s.prefix + prefix
	input := &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &fullPrefix,
	}
	if delimiter != "" {
		input.Delimiter = &delimiter
	}
	var result []string
	err := s.client.ListObjectsV2Pages(input,
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				name := strings.TrimPrefix(aws.StringValue(obj.Key), s.prefix)
				name = strings.TrimPrefix(name, prefix)
				result = append(result, name)
			}
			for _, cp := range page.CommonPrefixes {
				name := strings.TrimPrefix(aws.StringValue(cp.Prefix), s.prefix)
				name = strings.TrimPrefix(name, prefix)
				result = append(result, name)
			}
			return true
		},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "list prefix=%q delimiter=%q", prefix, delimiter)
	}
	return result, nil
}

// Delete implements remote.Storage.
func (s *S3Storage) Delete(objName string) error {
	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.objKey(objName)),
	})
	return errors.Wrapf(err, "delete object %q", objName)
}

// Size implements remote.Storage.
func (s *S3Storage) Size(objName string) (int64, error) {
	out, err := s.client.HeadObject(&s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.objKey(objName)),
	})
	if err != nil {
		return 0, errors.Wrapf(err, "head object %q", objName)
	}
	return aws.Int64Value(out.ContentLength), nil
}

// IsNotExistError implements remote.Storage.
func (s *S3Storage) IsNotExistError(err error) bool {
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case s3.ErrCodeNoSuchKey, "NotFound":
			return true
		}
	}
	return false
}

// Close implements remote.Storage.
func (s *S3Storage) Close() error {
	return nil
}

// s3ObjectReader implements remote.ObjectReader.
type s3ObjectReader struct {
	s       *S3Storage
	objName string
}

// ReadAt implements remote.ObjectReader.
func (r *s3ObjectReader) ReadAt(ctx context.Context, p []byte, offset int64) error {
	end := offset + int64(len(p)) - 1
	rangeHeader := aws.String(fmt.Sprintf("bytes=%d-%d", offset, end))
	out, err := r.s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: &r.s.bucket,
		Key:    aws.String(r.s.objKey(r.objName)),
		Range:  rangeHeader,
	})
	if err != nil {
		return errors.Wrapf(err, "read object %q at offset %d", r.objName, offset)
	}
	defer out.Body.Close()
	_, err = io.ReadFull(out.Body, p)
	return err
}

// Close implements remote.ObjectReader.
func (r *s3ObjectReader) Close() error {
	return nil
}

// s3ObjectWriter buffers writes and uploads on Close.
type s3ObjectWriter struct {
	s       *S3Storage
	objName string
	buf     bytes.Buffer
}

// Write implements io.Writer.
func (w *s3ObjectWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// Close implements io.Closer. Uploads the buffered data to S3.
func (w *s3ObjectWriter) Close() error {
	_, err := w.s.uploader.Upload(&s3manager.UploadInput{
		Bucket: &w.s.bucket,
		Key:    aws.String(w.s.objKey(w.objName)),
		Body:   bytes.NewReader(w.buf.Bytes()),
	})
	return errors.Wrapf(err, "upload object %q", w.objName)
}
