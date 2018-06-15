#!/usr/bin/env bash

set -ve

# This script:
# - assumes you've created a ssh key used by kube-aws-bot to ssh into github:
#   https://help.github.com/articles/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent/
# - utilizes Travis CI's file encryption feature for encrypting ssh keys

# It requires the following command beforehand:
#   $ gem install travis
#   $ travis login --auto
#   $ travis encrypt-file ci/kube-aws-bot-git-ssh-key --repo <your github user or organization>/kube-aws

# And then change this line to the one output from the `travis encrypt-file` command above
openssl aes-256-cbc -K $encrypted_514cf8442810_key -iv $encrypted_514cf8442810_iv -in kube-aws-bot-git-ssh-key.enc -out ci/kube-aws-bot-git-ssh-key -d

## Prevent the following error:
##   Permissions 0644 for '/home/travis/gopath/src/github.com/kubernetes-incubator/kube-aws/ci/kube-aws-bot-git-ssh-key' are too open.
##   ...
##   bad permissions: ignore key: /home/travis/gopath/src/github.com/kubernetes-incubator/kube-aws/ci/kube-aws-bot-git-ssh-key
chmod 600 ci/kube-aws-bot-git-ssh-key

# Finally run the following command to add the encrypted key to the git repo:
#   $ git add kube-aws-bot-git-ssh-key.enc
#   $   $ git commit -m '...'

echo -e "Host github.com\n\tStrictHostKeyChecking no\nIdentityFile $(pwd)/ci/kube-aws-bot-git-ssh-key\n" >> ~/.ssh/config

set +e
ssh git@github.com
status=$?
set -e

if [ $status -ne 1 ]; then
  echo ssh connection check to github failed: ssh command exited with status $status 1>&2
  exit 1
fi

echo Node.js $(node -v) is installed/being used

REPO=$TRAVIS_REPO_SLUG make publish-docs
