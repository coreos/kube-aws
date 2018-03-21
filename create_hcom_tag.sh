#! /bin/bash

usage() {
    >&2 cat <<EOF
Usage: $0 BUILD_NUMBER

BUILD_NUMBER - this is the number to use to append to the patch-version of the tag to ensure uniqueness

This script will grab the newest tag and then create a unique hcom one for use by downstream tools. The tag is linked back to the bamboo build which created it via supplied build_number, which will normally be the bamboo build_number.
EOF
  exit 1
}

[[ "$#" -eq 1 ]] || usage
BUILD_NUMBER=$1

LATEST_TAG=$(git tag -l v*.*.* --contains $(git rev-list --tags=v* --max-count=1))
echo "The latest tag found is: $LATEST_TAG"

if [[ ! "$LATEST_TAG" =~ ^v[0-9]+\.[0-9]+\.* ]]; then
	echo "The \"latest\" tag did not exist, or did not point to a commit which also had a semantic version tag."
	exit 1
fi

MAJOR=$(echo "$LATEST_TAG" | awk -F '.' '{print $1}')
MINOR=$(echo "$LATEST_TAG" | awk -F '.' '{print $2}')
PATCH=$(echo "$LATEST_TAG" | awk -F '.' '{print $3}' | awk -F '-' '{print $1}')

NEW_TAG="${MAJOR}.${MINOR}.${PATCH}-hcom.${BUILD_NUMBER}"

git tag -a "$NEW_TAG" -m "Version $NEW_TAG" > /dev/null

echo "Latest tags:"
git tag | tail -n5
