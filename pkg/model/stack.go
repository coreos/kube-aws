package model

import (
	"fmt"
	"net"
	"strings"

	"errors"

	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pki"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
	"net/url"
	"time"
)

// VERSION set by build script
var VERSION = "UNKNOWN"

const STACK_TEMPLATE_FILENAME = "stack.json"

// RenderAndAddUserData adds a userdata with the id that is loaded from the file located at `userdataTmplPath`.
// When the id is "Controller", the loaded useradata can be referenced by `Userdata.Controller` in templates.
func (s *Stack) RenderAndAddUserData(id, userdataTmplPath string) error {
	var err error

	id = strings.Title(id)

	if s.UserData == nil {
		s.UserData = map[string]api.UserData{}
	}

	s.UserData[id], err = api.NewUserDataFromTemplateFile(userdataTmplPath, s.tmplCtx)

	if err != nil {
		return fmt.Errorf("failed to render userdata: %v", err)
	}

	return nil
}

func (p *Stack) RenderAddControllerUserdata(opts api.StackTemplateOptions) error {
	return p.RenderAndAddUserData(
		"Controller",
		p.ControllerTmplFile,
	)
}

func (p *Stack) RenderAddEtcdUserdata(opts api.StackTemplateOptions) error {
	return p.RenderAndAddUserData(
		"Etcd",
		p.EtcdTmplFile,
	)
}

func (p *Stack) RenderAddWorkerUserdata(opts api.StackTemplateOptions) error {
	return p.RenderAndAddUserData(
		"Worker",
		p.WorkerTmplFile,
	)
}

func (c *Stack) Assets() cfnstack.Assets {
	if c.assets == nil {
		panic(fmt.Sprintf("[bug] encountered unexpected nil assets for stack %s", c.StackName))
	}
	return c.assets
}

func (c *Stack) buildAssets() (cfnstack.Assets, error) {
	logger.Debugf("Building assets for %s", c.StackName)
	logger.Debugf("Context is: %+v", c)

	var err error

	assetsBuilder, err := cfnstack.NewAssetsBuilder(c.StackName, c.ClusterExportedStacksS3URI(), c.Region)
	if err != nil {
		return nil, err
	}

	if err := c.addTarballedAssets(assetsBuilder); err != nil {
		return nil, fmt.Errorf("failed to create node provisioner: %v", err)
	}

	for id, _ := range c.UserData {
		userdataS3PartAssetName := "userdata-" + strings.ToLower(id)

		if err = assetsBuilder.AddUserDataPart(c.UserData[id], api.USERDATA_S3, userdataS3PartAssetName); err != nil {
			return nil, fmt.Errorf("failed to addd %s: %v", userdataS3PartAssetName, err)
		}
	}

	logger.Debugf("Buildings assets before templating %s stack template...", c.StackName)
	c.assets = assetsBuilder.Build()

	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("failed to render \"%s\" stack template: %v", c.StackName, err)
	}

	logger.Debugf("Calling assets.Add on %s", STACK_TEMPLATE_FILENAME)
	assetsBuilder.Add(STACK_TEMPLATE_FILENAME, stackTemplate)

	logger.Debugf("Calling assets.Build for %s...", c.StackName)
	return assetsBuilder.Build(), nil
}

func (s *Stack) addTarballedAssets(assetsBuilder *cfnstack.AssetsBuilderImpl) error {
	if len(s.archivedFiles) == 0 {
		return nil
	}

	t := time.Now()

	//s3Client := s3.New(s.Session)

	files := s.archivedFiles

	ts := t.Format("20060102150405")
	cacheDir := fmt.Sprintf("cache/%s/%s", ts, s.StackName)

	loader := &provisioner.RemoteFileLoader{}

	prov := provisioner.NewTarballingProvisioner(s.StackName, files, "", assetsBuilder.S3DirURI(), cacheDir)

	trans, err := prov.CreateTransferredFile(loader)
	if err != nil {
		return err
	}
	loaded, err := loader.Load(trans.RemoteFileSpec)
	if err != nil {
		return err
	}

	assetsBuilder.Add(prov.GetTransferredFile().BaseName(), loaded.Content.String())

	s.NodeProvisioner = prov

	return nil
}

func (c *Stack) TemplateURL() (string, error) {
	asset, err := c.assets.FindAssetByStackAndFileName(c.StackName, STACK_TEMPLATE_FILENAME)
	if err != nil {
		return "", fmt.Errorf("failed to get template URL: %v", err)
	}
	return asset.URL()
}

// NestedStackName returns a sanitized name of this control-plane which is usable as a valid cloudformation nested stack name
func (c Stack) NestedStackName() string {
	return naming.FromStackToCfnResource(c.StackName)
}

func (c *Stack) String() string {
	return fmt.Sprintf("{Config:%+v}", c.Config)
}

// validateCertsAgainstSettings cross checks that our api server cert is compatible with our cluster settings: -
// - It must include the externalDNS name for the api servers.
// - It must include the IPAddress of the first IP in the chosen ServiceCIDR.
func (c *Stack) validateCertsAgainstSettings() error {
	apiServerPEM, err := gzipcompressor.GzippedBase64StringToString(c.AssetsConfig.APIServerCert)
	if err != nil {
		return fmt.Errorf("could not decompress the apiserver pem: %v", err)
	}

	apiServerCerts, err := pki.CertificatesFromBytes([]byte(apiServerPEM))
	if err != nil {
		return fmt.Errorf("error parsing api server cert: %v", err)
	}
	kubeAPIServerCert, ok := apiServerCerts.GetBySubjectCommonNamePattern("kube-apiserver")
	if !ok {
		return errors.New("no api server certs contain Subject CommonName 'kube-apiserver'")
	}

	// Check DNS Names
	for _, apiEndPoint := range c.Config.KubeClusterSettings.APIEndpointConfigs {
		if !kubeAPIServerCert.ContainsDNSName(apiEndPoint.DNSName) {
			return fmt.Errorf("the apiserver cert does not contain the external dns name %s, please regenerate or resolve", apiEndPoint.DNSName)
		}
	}

	// Check IP SANS
	_, serviceNet, err := net.ParseCIDR(c.Config.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	kubernetesServiceIPAddr := netutil.IncrementIP(serviceNet.IP)

	if !kubeAPIServerCert.ContainsIPAddress(kubernetesServiceIPAddr) {
		return fmt.Errorf("the api server cert does not contain the kubernetes service ip address %v, please regenerate or resolve", kubernetesServiceIPAddr)
	}
	return nil
}

func (c *Stack) s3Folders() api.S3Folders {
	return api.NewS3Folders(c.S3URI, c.ClusterName)
}

func (c *Stack) ClusterS3URI() string {
	return c.s3Folders().Cluster().URI()
}

func (c *Stack) ClusterExportedStacksS3URI() string {
	return c.s3Folders().ClusterExportedStacks().URI()
}

// EtcdSnapshotsS3Path is a pair of a S3 bucket and a key of an S3 object containing an etcd cluster snapshot
func (c Stack) EtcdSnapshotsS3PathRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3PathRef : %v", err)
	}
	return fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, s3uri.Host, s3uri.Path), nil
}

func (c Stack) EtcdSnapshotsS3Bucket() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Bucket : %v", err)
	}
	return s3uri.Host, nil
}

func (c Stack) EtcdSnapshotsS3PrefixRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Prefix : %v", err)
	}
	s3path := fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, strings.TrimLeft(s3uri.Path, "/"))
	return s3path, nil
}

func (c *Stack) RenderStackTemplateAsBytes() ([]byte, error) {
	logger.Debugf("Template Context:-\n%+v\n", c)
	return jsontemplate.GetBytes(c.StackTemplateTmplFile, c.tmplCtx, c.PrettyPrint)
}

func (c *Stack) RenderStackTemplateAsString() (string, error) {
	logger.Debugf("Called RenderStackTemplateAsString on %s", c.StackName)
	bytes, err := c.RenderStackTemplateAsBytes()
	return string(bytes), err
}

func (c *Stack) GetUserData(id string) *api.UserData {
	id = strings.Title(id)
	if userdata, ok := c.UserData[id]; ok {
		return &userdata
	}
	return nil
}
