package cfnstack

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"github.com/kubernetes-incubator/kube-aws/model"
	"path/filepath"
	"strings"
)

type Assets interface {
	Merge(Assets) Assets
	AsMap() map[model.AssetID]model.Asset
	FindAssetByStackAndFileName(string, string) (model.Asset, error)
}

type assetsImpl struct {
	underlying map[model.AssetID]model.Asset
}

func (a assetsImpl) Merge(other Assets) Assets {
	merged := map[model.AssetID]model.Asset{}

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

func (a assetsImpl) AsMap() map[model.AssetID]model.Asset {
	return a.underlying
}

func (a assetsImpl) findAssetByID(id model.AssetID) (model.Asset, error) {
	asset, ok := a.underlying[id]
	if !ok {
		return asset, fmt.Errorf("[bug] failed to get the asset for the id \"%s\"", id)
	}
	return asset, nil
}

func (a assetsImpl) FindAssetByStackAndFileName(stack string, file string) (model.Asset, error) {
	return a.findAssetByID(model.NewAssetID(stack, file))
}

type AssetsBuilder interface {
	Add(filename string, content string) (model.Asset, error)
	AddUserDataPart(userdata model.UserData, part string, assetName string) error
	Build() Assets
}

type assetsBuilderImpl struct {
	locProvider AssetLocationProvider
	assets      map[model.AssetID]model.Asset
}

func (b *assetsBuilderImpl) Add(filename string, content string) (model.Asset, error) {
	loc, err := b.locProvider.locationFor(filename)
	if err != nil {
		return model.Asset{}, err
	}

	asset := model.Asset{
		AssetLocation: *loc,
		Content:       content,
	}

	b.assets[loc.ID] = asset
	return asset, nil
}

func (b *assetsBuilderImpl) AddUserDataPart(userdata model.UserData, part string, assetName string) error {
	if p, ok := userdata.Parts[part]; ok {
		content, err := p.Template()
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("%s-%s", assetName, fingerprint.SHA256(content))
		asset, err := b.Add(filename, content)
		if err != nil {
			return err
		}
		p.Asset = asset
	}
	return nil // it is not an error if part is not found
}

func (b *assetsBuilderImpl) Build() Assets {
	return assetsImpl{
		underlying: b.assets,
	}
}

func NewAssetsBuilder(stackName string, s3URI string, region model.Region) AssetsBuilder {
	return &assetsBuilderImpl{
		locProvider: AssetLocationProvider{
			s3URI:     s3URI,
			region:    region,
			stackName: stackName,
		},
		assets: map[model.AssetID]model.Asset{},
	}
}

type AssetLocationProvider struct {
	s3URI     string
	region    model.Region
	stackName string
}

func (p AssetLocationProvider) locationFor(filename string) (*model.AssetLocation, error) {
	if filename == "" {
		return nil, fmt.Errorf("Can't produce S3 location for empty filename")
	}
	s3URI := p.s3URI

	uri, err := S3URIFromString(s3URI)

	if err != nil {
		return nil, fmt.Errorf("failed to determine location for %s: %v", filename, err)
	}

	relativePathComponents := []string{
		p.stackName,
		filename,
	}

	key := strings.Join(
		append(uri.PathComponents(), relativePathComponents...),
		"/",
	)

	id := model.NewAssetID(p.stackName, filename)

	return &model.AssetLocation{
		ID:     id,
		Key:    key,
		Bucket: uri.Bucket(),
		Path:   filepath.Join(relativePathComponents...),
		Region: p.region,
	}, nil
}
