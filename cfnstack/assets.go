package cfnstack

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"path/filepath"
	"strings"
)

type Assets interface {
	Merge(Assets) Assets
	AsMap() map[api.AssetID]api.Asset
	FindAssetByStackAndFileName(string, string) (api.Asset, error)
	S3Prefix() string
}

func EmptyAssets() assetsImpl {
	return assetsImpl{
		underlying: map[api.AssetID]api.Asset{},
	}
}

type assetsImpl struct {
	s3Prefix   string
	underlying map[api.AssetID]api.Asset
}

func (a assetsImpl) Merge(other Assets) Assets {
	merged := map[api.AssetID]api.Asset{}

	for k, v := range a.underlying {
		merged[k] = v
	}
	for k, v := range other.AsMap() {
		merged[k] = v
	}

	return assetsImpl{
		s3Prefix:   other.S3Prefix(),
		underlying: merged,
	}
}

func (a assetsImpl) S3Prefix() string {
	return a.s3Prefix
}

func (a assetsImpl) AsMap() map[api.AssetID]api.Asset {
	return a.underlying
}

func (a assetsImpl) findAssetByID(id api.AssetID) (api.Asset, error) {
	asset, ok := a.underlying[id]
	if !ok {
		ks := []string{}
		for id, _ := range a.underlying {
			k := fmt.Sprintf("%s/%s", id.StackName, id.Filename)
			ks = append(ks, k)
		}
		logger.Debugf("dumping stored asset keys: %s", strings.Join(ks, ", "))
		return asset, fmt.Errorf("[bug] failed to get the asset for the id \"%s\"", id)
	}
	return asset, nil
}

func (a assetsImpl) FindAssetByStackAndFileName(stack string, file string) (api.Asset, error) {
	return a.findAssetByID(api.NewAssetID(stack, file))
}

type AssetsBuilder interface {
	Add(filename string, content string) (api.Asset, error)
	AddUserDataPart(userdata api.UserData, part string, assetName string) error
	Build() Assets
}

type AssetsBuilderImpl struct {
	AssetLocationProvider
	assets map[api.AssetID]api.Asset
}

func (b *AssetsBuilderImpl) Add(filename string, content string) (api.Asset, error) {
	loc, err := b.Locate(filename)
	if err != nil {
		return api.Asset{}, err
	}

	asset := api.Asset{
		AssetLocation: *loc,
		Content:       content,
	}

	b.assets[loc.ID] = asset
	return asset, nil
}

func (b *AssetsBuilderImpl) AddUserDataPart(userdata api.UserData, part string, assetName string) error {
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

func (b *AssetsBuilderImpl) Build() Assets {
	return assetsImpl{
		s3Prefix:   b.S3Prefix(),
		underlying: b.assets,
	}
}

func NewAssetsBuilder(stackName string, s3URI string, region api.Region) (*AssetsBuilderImpl, error) {
	uri, err := S3URIFromString(s3URI)

	if err != nil {
		return nil, fmt.Errorf("failed creating s3 assets locator for stack %s: %v", stackName, err)
	}

	return &AssetsBuilderImpl{
		AssetLocationProvider: AssetLocationProvider{
			s3URI:     uri,
			region:    region,
			stackName: stackName,
		},
		assets: map[api.AssetID]api.Asset{},
	}, nil
}

type AssetLocationProvider struct {
	s3URI     S3URI
	region    api.Region
	stackName string
}

func (p AssetLocationProvider) S3DirURI() string {
	return fmt.Sprintf("%s/%s", p.s3URI.String(), p.stackName)
}

func (p AssetLocationProvider) Locate(filename string) (*api.AssetLocation, error) {
	if filename == "" {
		return nil, fmt.Errorf("Can't produce S3 location for empty filename")
	}

	relativePathComponents := []string{
		p.stackName,
		filename,
	}

	// key = s3uri's path component + stack name + filename
	key := strings.Join(
		append(p.s3URI.KeyComponents(), relativePathComponents...),
		"/",
	)

	id := api.NewAssetID(p.stackName, filename)

	return &api.AssetLocation{
		ID:     id,
		Key:    key,
		Bucket: p.s3URI.Bucket(),
		Path:   filepath.Join(relativePathComponents...),
		Region: p.region,
	}, nil
}

// S3Prefix returns BUCKET + / + S3 OBJECT KEY PREFIX whereas the prefix is that of all the assets locatable by this provider
// For example, in case this provider is configured to locate assets for stack MYSTACK in S3 bucket MYBUCKET
// due to that you've passed an S3 URI of `s3://MYBUCKET/MY/PREFIX` and the stack name of MYSTACK,
// this func returns "MYBUCKET/MY/PREFIX/MYSTACK".
func (p AssetLocationProvider) S3Prefix() string {
	return fmt.Sprintf("%s/%s", p.s3URI.BucketAndKey(), p.stackName)
}
