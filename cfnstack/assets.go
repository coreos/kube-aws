package cfnstack

import (
	"fmt"
	"path/filepath"
	"strings"
	"log"
)

type Assets interface {
	Merge(Assets) Assets
	AsMap() map[AssetID]Asset
	FindAssetByStackAndFileName(string, string) (Asset, error)
}

type assetsImpl struct {
	underlying map[AssetID]Asset
}

type AssetID interface {
	StackName() string
	Filename() string
}

type assetIDImpl struct {
	stackName string
	filename  string
}

func (i assetIDImpl) StackName() string {
	return i.stackName
}

func (i assetIDImpl) Filename() string {
	return i.filename
}

func NewAssetID(stack string, file string) AssetID {
	return assetIDImpl{
		stackName: stack,
		filename:  file,
	}
}

func (a assetsImpl) Merge(other Assets) Assets {
	merged := map[AssetID]Asset{}

	for k, v := range a.underlying {
		merged[k] = v
	}
	for k, v := range other.AsMap() {
		merged[k] = v
	}

	return assetsImpl{
		underlying: merged,
	}
}

func (a assetsImpl) AsMap() map[AssetID]Asset {
	return a.underlying
}

func (a assetsImpl) findAssetByID(id AssetID) (Asset, error) {
	asset, ok := a.underlying[id]
	if !ok {
		return asset, fmt.Errorf("[bug] failed to get the asset for the id \"%s\"", id)
	}
	return asset, nil
}

func (a assetsImpl) FindAssetByStackAndFileName(stack string, file string) (Asset, error) {
	return a.findAssetByID(NewAssetID(stack, file))
}

type AssetsBuilder interface {
	Add(filename string, content string) AssetsBuilder
	Build() Assets
}

type assetsBuilderImpl struct {
	locProvider AssetLocationProvider
	assets      map[AssetID]Asset
}

func (b *assetsBuilderImpl) Add(filename string, content string) AssetsBuilder {
	loc, err := b.locProvider.locationFor(filename)
	if err != nil {
		panic(err)
	}
	b.assets[loc.ID] = Asset{
		AssetLocation: *loc,
		Content:       content,
	}
	return b
}

func (b *assetsBuilderImpl) Build() Assets {
	return assetsImpl{
		underlying: b.assets,
	}
}

func NewAssetsBuilder(stackName string, s3URI string, s3Region string) AssetsBuilder {
	return &assetsBuilderImpl{
		locProvider: AssetLocationProvider{
			s3URI:     s3URI,
			s3Region:  s3Region,
			stackName: stackName,
		},
		assets: map[AssetID]Asset{},
	}
}

type Asset struct {
	AssetLocation
	Content string
}

type AssetLocationProvider struct {
	s3URI     string
	s3Region  string
	stackName string
}

type AssetLocation struct {
	ID       AssetID
	Key      string
	Bucket   string
	Path     string
	S3Region string
}

func (l AssetLocation) URL() string {
	var url string
	if strings.HasPrefix(l.S3Region, "cn") {
		url = fmt.Sprintf("https://s3.cn-north-1.amazonaws.com.cn/%s/%s", l.Bucket, l.Key)
	} else {
		url = fmt.Sprintf("https://s3.amazonaws.com/%s/%s", l.Bucket, l.Key)
	}
	log.Printf("Using S3 URL: %s on region: %s", url, l.S3Region)
	return url
}

func newAssetLocationProvider(stackName string, s3URI string, s3Region string) AssetLocationProvider {
	return AssetLocationProvider{
		s3URI:     s3URI,
		s3Region:  s3Region,
		stackName: stackName,
	}
}

func (p AssetLocationProvider) locationFor(filename string) (*AssetLocation, error) {
	s3URI := p.s3URI

	uri, err := S3URIFromString(s3URI)

	if err != nil {
		return nil, fmt.Errorf("failed to determin location for %s: %v", filename, err)
	}

	relativePathComponents := []string{
		p.stackName,
		filename,
	}

	key := strings.Join(
		append(uri.PathComponents(), relativePathComponents...),
		"/",
	)

	id := NewAssetID(p.stackName, filename)

	return &AssetLocation{
		ID:       id,
		Key:      key,
		Bucket:   uri.Bucket(),
		Path:     filepath.Join(relativePathComponents...),
		S3Region: p.s3Region,
	}, nil
}
