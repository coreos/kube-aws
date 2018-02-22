## Release note gathering

To generate a release note for the release `v0.9.8-rc.1` in Markdown:


The pre-requisite is to run `go get` several times to install dependencies:

```
$ go get -u "github.com/google/go-github/github"
$ go get -u "golang.org/x/oauth2"
$ go get -u "golang.org/x/context"
```

For a release note for a release-candidate version of kube-aws, run:

```
$ VERSION=v0.9.8-rc.1 go run relnotes.go
```

For one for a final version, run:

```
$ VERSION=v0.9.8 go run relnotes.go
```
