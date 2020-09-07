package s3

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/ppc64le-cloud/build-bot/pkg/storage"
)

var _ storage.Storage = &S3{}

type S3 struct{
	credentials *Credentials
	sess *session.Session
	uploader *s3manager.Uploader
	downloader *s3manager.Downloader
}

type Credentials struct {
	Region           string `json:"region"`
	Endpoint         string `json:"endpoint"`
	Insecure         bool   `json:"insecure"`
	S3ForcePathStyle bool   `json:"s3_force_path_style"`
	AccessKey        string `json:"access_key"`
	SecretKey        string `json:"secret_key"`
}

func NewSession(s3Credentials *Credentials) (*S3, error){
	var staticCredentials credentials.StaticProvider
	if s3Credentials.AccessKey != "" && s3Credentials.SecretKey != "" {
		staticCredentials = credentials.StaticProvider{
			Value: credentials.Value{
				AccessKeyID:     s3Credentials.AccessKey,
				SecretAccessKey: s3Credentials.SecretKey,
			},
		}
	}
	credentialChain := credentials.NewChainCredentials(
		[]credentials.Provider{
			&staticCredentials,
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(session.New()),
			},
		})
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentialChain,
		Endpoint:         aws.String(s3Credentials.Endpoint),
		DisableSSL:       aws.Bool(s3Credentials.Insecure),
		S3ForcePathStyle: aws.Bool(s3Credentials.S3ForcePathStyle),
		Region:           aws.String(s3Credentials.Region),
	})
	if err != nil {
		fmt.Printf("error creating S3 Session: %v", err)
	}

	return &S3 {
		s3Credentials,
		sess,
		s3manager.NewUploader(sess),
		s3manager.NewDownloader(sess),
	},  nil
}

func (s *S3) Download(bucket, object string) ([]byte, error) {
	wb := aws.WriteAtBuffer{}

	// Write the contents of S3 Object to the WriteAtBuffer
	n, err := s.downloader.Download(&wb, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file, %v", err)
	}
	fmt.Printf("file downloaded, %d bytes\n", n)
	return wb.Bytes(), nil
}

func (s *S3) Upload(bucket string, object string, content []byte) error {
	result, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   bytes.NewReader(content),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	fmt.Printf("file uploaded to, %s\n", aws.StringValue(&result.Location))
	return nil
}