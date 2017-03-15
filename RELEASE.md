# Release Process

kube-aws is released once all the issues in a GitHub milestone are resolved. The process is as follows:

1. All the issues in the next release milestone is resolved
2. An OWNER writes a draft of a GitHub release
3. An OWNER runs `git tag -s $VERSION`, and then `./containerized-build-release-binaries/` to produce released binaries
4. The OWNER uploads the released binaries to the draft, and then pushes the tag with `git push $VERSION`
5. The release milestone is closed
6. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the subject `[ANNOUNCE] kube-aws $VERSION is released`
