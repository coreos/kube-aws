package api

import (
	"fmt"
)

type Image struct {
	Repo          string `yaml:"repo,omitempty"`
	RktPullDocker bool   `yaml:"rktPullDocker,omitempty"`
	Tag           string `yaml:"tag,omitempty"`
}

func (i *Image) MergeIfEmpty(other Image) {
	if i.Repo == "" || i.Tag == "" {
		i.Repo = other.Repo
		i.Tag = other.Tag
		i.RktPullDocker = other.RktPullDocker
	}
}

func (i *Image) Options() string {
	if i.RktPullDocker {
		return "--insecure-options=image "
	}
	return ""
}

func (i *Image) RktRepo() string {
	if i.RktPullDocker {
		return fmt.Sprintf("docker://%s:%s", i.Repo, i.Tag)
	}
	return fmt.Sprintf("%s:%s", i.Repo, i.Tag)
}

func (i *Image) RktRepoWithoutTag() string {
	if i.RktPullDocker {
		return fmt.Sprintf("docker://%s", i.Repo)
	}
	return i.Repo
}

func (i *Image) RepoWithTag() string {
	return fmt.Sprintf("%s:%s", i.Repo, i.Tag)
}
