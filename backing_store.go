package main

import (
	"encoding/json"
	"fmt"
	"github.com/ppc64le-cloud/build-bot/pkg/storage/s3"
	"io/ioutil"
)

const (
	myBucket = "ppc64le-ci-builds"
)

var (
	sess *s3.S3
)

func backingStoreInit() error {
	if s3CredentialsFile == "" {
		return fmt.Errorf("--s3-credentials-file is missing")
	}
	s3Credentials, err := ioutil.ReadFile(s3CredentialsFile)
	if err != nil {
		return err
	}

	var cred s3.Credentials
	err = json.Unmarshal(s3Credentials, &cred)
	if err != nil {
		return err
	}
	sess, err = s3.NewSession(&cred)
	if err != nil {
		return fmt.Errorf("failed to create storage session, check the credentials")
	}
	return nil
}
