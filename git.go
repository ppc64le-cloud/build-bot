package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/klog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

var (
	client *github.Client
	ctx    = context.Background()
)

func getRef(baseBranch, commitBranch string) (ref *github.Reference, err error) {
	if ref, _, err = client.Git.GetRef(ctx, owner, repo, "refs/heads/"+commitBranch); err == nil {
		return ref, nil
	}

	var baseRef *github.Reference
	if baseRef, _, err = client.Git.GetRef(ctx, owner, repo, "refs/heads/"+baseBranch); err != nil {
		return nil, err
	}
	newRef := &github.Reference{Ref: github.String("refs/heads/" + commitBranch), Object: &github.GitObject{SHA: baseRef.Object.SHA}}
	ref, _, err = client.Git.CreateRef(ctx, owner, repo, newRef)
	return ref, err
}

func getTree(ref *github.Reference, sourceFiles string) (tree *github.Tree, err error) {
	var entries []*github.TreeEntry

	// Load each file into the tree.
	for _, fileArg := range strings.Split(sourceFiles, ",") {
		file, content, err := getFileContent(fileArg)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &github.TreeEntry{Path: github.String(file), Type: github.String("blob"), Content: github.String(string(content)), Mode: github.String("100644")})
	}

	tree, _, err = client.Git.CreateTree(ctx, owner, repo, *ref.Object.SHA, entries)
	return tree, err
}

func getFileContent(fileArg string) (targetName string, b []byte, err error) {
	var localFile string
	files := strings.Split(fileArg, ":")
	switch {
	case len(files) < 1:
		return "", nil, errors.New("empty files parameter")
	case len(files) == 1:
		localFile = files[0]
		targetName = files[0]
	default:
		localFile = files[0]
		targetName = files[1]
	}

	b, err = ioutil.ReadFile(localFile)
	return targetName, b, err
}

func pushCommit(ref *github.Reference, tree *github.Tree, commitMessage *string) (err error) {
	parent, _, err := client.Repositories.GetCommit(ctx, owner, repo, *ref.Object.SHA)
	if err != nil {
		return err
	}
	// This is not always populated, but is needed.
	parent.Commit.SHA = parent.SHA

	// Create the commit using the tree.
	date := time.Now()
	author := &github.CommitAuthor{Date: &date, Name: &authorName, Email: &authorEmail}
	commit := &github.Commit{Author: author, Message: commitMessage, Tree: tree, Parents: []*github.Commit{parent.Commit}}
	newCommit, _, err := client.Git.CreateCommit(ctx, owner, repo, commit)
	if err != nil {
		return err
	}

	// Attach the commit to the master branch.
	ref.Object.SHA = newCommit.SHA
	_, _, err = client.Git.UpdateRef(ctx, owner, repo, ref, false)
	return err
}

func createPR(prSubject, prDescription, prBranch, commitBranch *string) (err error) {

	newPR := &github.NewPullRequest{
		Title:               prSubject,
		Head:                commitBranch,
		Base:                prBranch,
		Body:                prDescription,
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s\n", pr.GetHTMLURL())
	_, _, err = client.PullRequests.Merge(ctx, owner, repo, *pr.Number, "Merged!", &github.PullRequestOptions{})
	if err != nil {
		return err
	}

	_, err = client.Git.DeleteRef(ctx, owner, repo, fmt.Sprintf("refs/heads/%s", *commitBranch))
	if err != nil {
		return err
	}
	return nil
}

func handleGetBuild(w http.ResponseWriter, req *http.Request) error {
	params := req.URL.Query()
	project := params.Get("project")
	commit := params.Get("commit")

	if project == "" || commit == "" {
		return fmt.Errorf("project or commit param is missing")
	}

	if commit != "" {
		content, err := sess.Download(myBucket, fmt.Sprintf("%s/%s/build.yaml", project, commit))
		if err != nil {
			return fmt.Errorf("failed to get %s/%s/build.yaml from the bucket", project, commit)
		}
		var build Build
		err = json.Unmarshal(content, &build)
		if err != nil {
			return fmt.Errorf("failed to unmarshal the build.yaml content to Build structure")
		}
		content, err = sess.Download(myBucket, fmt.Sprintf("%s/%s/%s", project, commit, build.Artifacts))
		if err != nil {
			return fmt.Errorf("failed to get %s/%s/%s from the bucket", project, commit, build.Artifacts)
		}

		contentType := http.DetectContentType(content)

		size := strconv.FormatInt(int64(len(content)), 10)

		//Send the headers
		w.Header().Set("Content-Disposition", "attachment; filename="+build.Artifacts[0])
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", size)

		io.Copy(w, bytes.NewReader(content))
	}
	return nil
}

func getFromFile(keyname string, req *http.Request) ([]byte, string, error) {
	var buf bytes.Buffer
	// in your case formFile would be fileupload
	formFile, header, err := req.FormFile(keyname)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get the %s from formFile", keyname)
	}
	defer formFile.Close()
	io.Copy(&buf, formFile)
	defer buf.Reset()
	return buf.Bytes(), header.Filename, nil
}

func handlePostBuild(req *http.Request) error {
	klog.Infoln("Entry: handlePostBuild")
	klog.Infoln("Parsing ParseMultipartForm")
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		klog.Errorf("failed to ParseMultipartForm")
		return fmt.Errorf("failed to ParseMultipartForm")
	}
	for key, _ := range req.Form {
		klog.Infof("%s = %s\n", key, req.Form.Get(key))
	}

	if req.Form.Get("source") == "" || req.Form.Get("commit") == "" || req.Form.Get("project") == "" {
		return fmt.Errorf("source, commit and project are the must have form params")
	}
	params := req.URL.Query()
	dryrun := params.Get("dryrun")

	var files []string
	for fl := range req.MultipartForm.File {
		content, filename, err := getFromFile(fl, req)
		if err != nil {
			return fmt.Errorf("failed to get the file content: %v", err)
		}
		klog.Infof("file found in the request: %s and the actual filename is %s", fl, filename)
		files = append(files, filename)
		if dryrun != "" {
			continue
		}
		err = sess.Upload(myBucket, fmt.Sprintf("%s/%s/%s", req.Form.Get("project"), req.Form.Get("commit"), filename), content)
		if err != nil {
			return fmt.Errorf("failed to upload the file: %s to S3 object store", fl)
		}
	}
	b := Build{
		req.Form.Get("source"),
		req.Form.Get("commit"),
		files,
		req.Form.Get("project"),
	}
	if dryrun != "" {
		klog.Infof("Build: %v", b)
		return nil
	}
	tmpfs, err := ioutil.TempFile("", "build")
	if err != nil {
		return fmt.Errorf("failed to create a tempfile: %v", err)
	}
	defer os.Remove(tmpfs.Name())

	bytes, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal build struct: %v", err)
	}

	// Uploading the build.yaml to the bucket
	err = sess.Upload(myBucket, fmt.Sprintf("%s/%s/%s", req.Form.Get("project"), req.Form.Get("commit"), "build.yaml"), bytes)
	if err != nil {
		return fmt.Errorf("failed to upload the content to S3 object store")
	}

	_, err = tmpfs.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write build content to formFile: %v", err)
	}

	if err := tmpfs.Close(); err != nil {
		return fmt.Errorf("failed to close the formFile: %v", err)
	}

	file := fmt.Sprintf("%s:%s/build.yaml", tmpfs.Name(), b.Project)
	commitMessage := fmt.Sprintf("%s commit %s", b.Project, b.Commit)
	commitBranch := "br1"
	baseBranch := "master"
	prSubject := fmt.Sprintf("Update %s build", b.Project)
	prDescription := "This is an automated PR via build-bot"

	ref, err := getRef(baseBranch, commitBranch)
	if err != nil {
		return err
	}
	tree, err := getTree(ref, file)
	if err != nil {
		return err
	}

	if err := pushCommit(ref, tree, &commitMessage); err != nil {
		return fmt.Errorf("Unable to create the commit: %s\n", err)
	}

	if err := createPR(&prSubject, &prDescription, &baseBranch, &commitBranch); err != nil {
		return fmt.Errorf("Error while creating the pull request: %s", err)
	}
	klog.Infoln("Exit: handlePostBuild")
	return nil
}

func gitInit() error {
	var token string
	// override the token set in the GITHUB_AUTH_TOKEN env if any
	if tkn := os.Getenv("GITHUB_AUTH_TOKEN"); tkn != "" {
		token = tkn
	} else {
		if tokenPath == "" {
			return fmt.Errorf("github token is missing, either --github-token-path or GITHUB_AUTH_TOKEN env is missing")
		}
		secret, err := ioutil.ReadFile(tokenPath)
		if err != nil {
			return err
		}
		token = string(bytes.TrimSpace(secret))
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client = github.NewClient(tc)
	return nil
}
