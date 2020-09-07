package main

import (
	"flag"
	"k8s.io/klog/v2"
)

var (
	owner             string
	repo              string
	authorName        = "Powercloud Bot"
	authorEmail       = "ltccci@in.ibm.com"
	s3CredentialsFile string
	tokenPath string
)

func init() {
	flag.StringVar(&owner, "owner", "ppc64le-cloud", "GH Org")
	flag.StringVar(&repo, "repo", "builds", "GH Repo")
	flag.StringVar(&s3CredentialsFile, "s3-credentials-file", "", "File where s3 credentials are stored. For the exact format see https://github.com/kubernetes/test-infra/blob/master/prow/io/providers/providers.go")
	flag.StringVar(&tokenPath, "github-token-path", "", "Path to the file containing the GitHub OAuth secret.")
	flag.Set("logtostderr", "false")
	flag.Set("log_file", "myfile.log")
	flag.Parse()
	klog.InitFlags(nil)
	if err := backingStoreInit(); err != nil {
		klog.Fatalf("failed to backingStoreInit: %v", err)
	}
	if err := gitInit(); err != nil {
		klog.Fatalf("failed to gitInit: %v", err)
	}
}
