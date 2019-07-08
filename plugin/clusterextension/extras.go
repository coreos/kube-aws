package clusterextension

import (
	"fmt"

	"encoding/json"

	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/plugin/plugincontents"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginutil"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
	"github.com/kubernetes-incubator/kube-aws/tmpl"

	//"os"
	"path/filepath"

	"github.com/kubernetes-incubator/kube-aws/logger"
)

type ClusterExtension struct {
	plugins []*api.Plugin
	Configs api.PluginConfigs
}

func NewExtrasFromPlugins(plugins []*api.Plugin, configs api.PluginConfigs) ClusterExtension {
	return ClusterExtension{
		plugins: plugins,
		Configs: configs,
	}
}

func NewExtras() ClusterExtension {
	return ClusterExtension{
		plugins: []*api.Plugin{},
		Configs: api.PluginConfigs{},
	}
}

type stack struct {
	Resources map[string]interface{}
	Outputs   map[string]interface{}
	Tags      map[string]interface{}
}

func (e ClusterExtension) KeyPairSpecs() []api.KeyPairSpec {
	keypairs := []api.KeyPairSpec{}
	err := e.foreachEnabledPlugins(func(p *api.Plugin, pc *api.PluginConfig) error {
		for _, spec := range p.Spec.Cluster.PKI.KeyPairs {
			keypairs = append(keypairs, spec)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return keypairs
}

func (e ClusterExtension) RootStack(config interface{}) (*stack, error) {
	logger.Debugf("Generating Plugin extras for root cloudformation stack")
	return e.stackExt("root", config, func(p *api.Plugin) api.Stack {
		return p.Spec.Cluster.CloudFormation.Stacks.Root
	})
}

func (e ClusterExtension) NetworkStack(config interface{}) (*stack, error) {
	logger.Debugf("Generating Plugin extras for network cloudformation stack")
	return e.stackExt("network", config, func(p *api.Plugin) api.Stack {
		return p.Spec.Cluster.CloudFormation.Stacks.Network
	})
}

type worker struct {
	ArchivedFiles       []provisioner.RemoteFileSpec
	CfnInitConfigSets   map[string]interface{}
	Files               []api.CustomFile
	SystemdUnits        []api.CustomSystemdUnit
	IAMPolicyStatements []api.IAMPolicyStatement
	NodeLabels          api.NodeLabels
	FeatureGates        api.FeatureGates
	Kubeconfig          string
	KubeletFlags        api.CommandLineFlags
	KubeletVolumeMounts []api.ContainerVolumeMount
}

type controller struct {
	ArchivedFiles       []provisioner.RemoteFileSpec
	APIServerFlags      api.CommandLineFlags
	APIServerVolumes    api.APIServerVolumes
	ControllerFlags     api.CommandLineFlags
	KubeProxyConfig     map[string]interface{}
	KubeSchedulerFlags  api.CommandLineFlags
	KubeletFlags        api.CommandLineFlags
	CfnInitConfigSets   map[string]interface{}
	Files               []api.CustomFile
	SystemdUnits        []api.CustomSystemdUnit
	IAMPolicyStatements []api.IAMPolicyStatement
	NodeLabels          api.NodeLabels
	Kubeconfig          string
	KubeletVolumeMounts []api.ContainerVolumeMount

	KubernetesManifestFiles []*provisioner.RemoteFile
	HelmReleaseFilesets     []api.HelmReleaseFileset
}

type etcd struct {
	Files               []api.CustomFile
	SystemdUnits        []api.CustomSystemdUnit
	IAMPolicyStatements []api.IAMPolicyStatement
}

func (e ClusterExtension) foreachEnabledPlugins(do func(p *api.Plugin, pc *api.PluginConfig) error) error {
	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.Configs); enabled {
			if err := do(p, pc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e ClusterExtension) stackExt(name string, config interface{}, src func(p *api.Plugin) api.Stack) (*stack, error) {
	resources := map[string]interface{}{}
	outputs := map[string]interface{}{}
	tags := map[string]interface{}{}

	err := e.foreachEnabledPlugins(func(p *api.Plugin, pc *api.PluginConfig) error {
		logger.Debugf("extras.go stackExt() foreachEnabledPlugins extending stack %s: %+v into %+v", name, pc.Values, p.Spec.Cluster.Values)
		values, err := pluginutil.MergeValues(p.Spec.Cluster.Values, pc.Values)
		if err != nil {
			return err
		}
		logger.Debugf("extras.go stackExt() resultant values: %+v", values)

		render := plugincontents.NewTemplateRenderer(p, values, config)

		m, err := render.MapFromJsonContents(src(p).Resources.RemoteFileSpec)
		if err != nil {
			return fmt.Errorf("failed to load additional resources for %s stack: %v", name, err)
		}
		for k, v := range m {
			resources[k] = v
		}

		m, err = render.MapFromJsonContents(src(p).Outputs.RemoteFileSpec)
		if err != nil {
			return fmt.Errorf("failed to load additional outputs for %s stack: %v", name, err)
		}
		for k, v := range m {
			outputs[k] = v
		}

		m, err = render.MapFromJsonContents(src(p).Tags.RemoteFileSpec)
		if err != nil {
			return fmt.Errorf("failed to load additional tags for %s stack: %v", name, err)
		}
		for k, v := range m {
			tags[k] = v
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Debugf("PLUGINS: StackExt Additions for stack %s", name)
	logger.Debugf("Resources: %+v", resources)
	logger.Debugf("Outputs: %+v", outputs)
	logger.Debugf("Tags: %+v", tags)

	return &stack{
		Resources: resources,
		Outputs:   outputs,
		Tags:      tags,
	}, nil
}

func (e ClusterExtension) NodePoolStack(config interface{}) (*stack, error) {
	logger.Debugf("Generating Plugin extras for nodepool cloudformation stack")
	return e.stackExt("node-pool", config, func(p *api.Plugin) api.Stack {
		return p.Spec.Cluster.CloudFormation.Stacks.NodePool
	})
}

func renderMachineSystemdUnits(r *plugincontents.TemplateRenderer, systemd api.SystemdUnits) ([]api.CustomSystemdUnit, error) {
	result := []api.CustomSystemdUnit{}

	for _, d := range systemd {
		s, err := r.File(d.RemoteFileSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to load systemd unit: %v", err)
		}

		u := api.CustomSystemdUnit{
			Name:    d.Name,
			Command: "start",
			Content: s,
			Enable:  true,
			Runtime: false,
		}
		result = append(result, u)
	}

	return result, nil
}

// simpleRenderMachineFiles - iterates over a plugin's MachineSpec.Files section loading/rendering contents to produce
// a list of custom files with their contents filled-in with the rendered results.
// NOTE1 - it does not use api.CustomFile's own template functionality (it is pre-rendered in the context of the plugin)
// NOTE2 - it knows nothing about binary files or configset files.
func simpleRenderMachineFiles(r *plugincontents.TemplateRenderer, files api.Files) ([]api.CustomFile, error) {
	result := []api.CustomFile{}

	for _, d := range files {
		s, err := r.File(d)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin etcd file contents: %v", err)
		}
		perm := d.Permissions
		f := api.CustomFile{
			Path:        d.Path,
			Permissions: perm,
			Content:     s,
		}
		result = append(result, f)
	}

	return result, nil
}

// renderMachineFilesAndConfigSets - the more complex partner of simpleRenderMachineFiles which also caters for binary files and configset files
func renderMachineFilesAndConfigSets(r *plugincontents.TemplateRenderer, files api.Files) ([]provisioner.RemoteFileSpec, []api.CustomFile, map[string]interface{}, error) {
	archivedFiles := []provisioner.RemoteFileSpec{}
	regularFiles := []api.CustomFile{}
	configsetFiles := make(map[string]interface{})

	for _, d := range files {
		if d.IsBinary() {
			archivedFiles = append(archivedFiles, d)
			continue
		}

		s, cfg, err := regularOrConfigSetFile(d, r)
		if err != nil {
			return archivedFiles, regularFiles, configsetFiles, fmt.Errorf("failed to load plugin worker file contents: %v", err)
		}

		var perm uint
		perm = d.Permissions

		if s != nil {
			f := api.CustomFile{
				Path:        d.Path,
				Permissions: perm,
				Content:     *s,
				Type:        d.Type,
			}
			regularFiles = append(regularFiles, f)
		} else {
			configsetFiles[d.Path] = map[string]interface{}{
				"content": cfg,
			}
		}
	}
	return archivedFiles, regularFiles, configsetFiles, nil
}

func regularOrConfigSetFile(f provisioner.RemoteFileSpec, render *plugincontents.TemplateRenderer) (*string, map[string]interface{}, error) {
	logger.Debugf("regularOrConfigSetFile(): Reading remoteFileSpec: %+v", f)
	goRendered, err := render.File(f)
	if err != nil {
		return nil, nil, err
	}

	// Disable templates for files that are known to be static e.g. binaries and credentials
	// Type "binary" is not considered here because binaries are too huge to be handled here.
	if f.Type == "credential" {
		return &goRendered, nil, nil
	}

	tokens := tmpl.TextToCfnExprTokens(goRendered)

	logger.Debugf("Number of tokens produced: %d", len(tokens))
	if len(tokens) == 1 {
		var out string
		if err := json.Unmarshal([]byte(tokens[0]), &out); err != nil {
			logger.Errorf("failed unmarshalling %s from json: %v", tokens[0], err)
			return nil, nil, err
		}
		return &out, nil, nil
	}

	return nil, map[string]interface{}{"Fn::Join": []interface{}{"", tokens}}, nil
}

func (e ClusterExtension) Worker(config interface{}) (*worker, error) {
	logger.Debugf("ClusterExtension.Worker(): Generating Plugin Worker user-data extras")
	files := []api.CustomFile{}
	systemdUnits := []api.CustomSystemdUnit{}
	iamStatements := []api.IAMPolicyStatement{}
	nodeLabels := api.NodeLabels{}
	featureGates := api.FeatureGates{}
	configsets := map[string]interface{}{}
	archivedFiles := []provisioner.RemoteFileSpec{}
	var kubeconfig string
	kubeletFlags := api.CommandLineFlags{}
	kubeletMounts := []api.ContainerVolumeMount{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.Configs); enabled {
			values, err := pluginutil.MergeValues(p.Spec.Cluster.Values, pc.Values)
			if err != nil {
				return nil, err
			}
			render := plugincontents.NewTemplateRenderer(p, values, config)

			extraUnits, err := renderMachineSystemdUnits(render, p.Spec.Cluster.Machine.Roles.Worker.Systemd.Units)
			if err != nil {
				return nil, fmt.Errorf("failed adding systemd units to worker: %v", err)
			}
			systemdUnits = append(systemdUnits, extraUnits...)

			extraArchivedFiles, extraFiles, extraConfigSetFiles, err := renderMachineFilesAndConfigSets(render, p.Spec.Cluster.Roles.Worker.Files)
			if err != nil {
				return nil, fmt.Errorf("failed adding files to worker: %v", err)
			}
			archivedFiles = append(archivedFiles, extraArchivedFiles...)
			files = append(files, extraFiles...)
			configsets[p.Name] = map[string]map[string]interface{}{
				"files": extraConfigSetFiles,
			}

			iamStatements = append(iamStatements, p.Spec.Cluster.Machine.Roles.Worker.IAM.Policy.Statements...)

			for k, v := range p.Spec.Cluster.Machine.Roles.Worker.Kubelet.NodeLabels {
				nodeLabels[k] = v
			}

			for k, v := range p.Spec.Cluster.Machine.Roles.Worker.Kubelet.FeatureGates {
				featureGates[k] = v
			}

			if p.Spec.Cluster.Machine.Roles.Worker.Kubelet.Kubeconfig != "" {
				kubeconfig = p.Spec.Cluster.Machine.Roles.Worker.Kubelet.Kubeconfig
			}

			if len(p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Mounts) > 0 {
				kubeletMounts = append(kubeletMounts, p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Mounts...)
			}

			extraKubeletFlags, err := getFlags(render, p.Spec.Cluster.Kubernetes.Kubelet.Flags)
			if err != nil {
				return nil, err
			}
			kubeletFlags = append(kubeletFlags, extraKubeletFlags...)
		}
	}

	return &worker{
		ArchivedFiles:       archivedFiles,
		CfnInitConfigSets:   configsets,
		Files:               files,
		SystemdUnits:        systemdUnits,
		IAMPolicyStatements: iamStatements,
		NodeLabels:          nodeLabels,
		FeatureGates:        featureGates,
		KubeletVolumeMounts: kubeletMounts,
		KubeletFlags:        kubeletFlags,
		Kubeconfig:          kubeconfig,
	}, nil
}

func (e ClusterExtension) ControlPlaneStack(config interface{}) (*stack, error) {
	logger.Debugf("Generating Plugin extras for control-plane cloudformation stack")
	return e.stackExt("control-plane", config, func(p *api.Plugin) api.Stack {
		return p.Spec.Cluster.CloudFormation.Stacks.ControlPlane
	})
}

func (e ClusterExtension) EtcdStack(config interface{}) (*stack, error) {
	logger.Debugf("Generating Plugin extras for etcd cloudformation stack")
	return e.stackExt("etcd", config, func(p *api.Plugin) api.Stack {
		return p.Spec.Cluster.CloudFormation.Stacks.Etcd
	})
}

// renderKubernetesManifests - yet another specialised function for rendering provisioner.RemoteFileSpec this time into kubernetes manifests
func renderKubernetesManifests(pluginName string, r *plugincontents.TemplateRenderer, mspecs api.KubernetesManifests) ([]api.CustomFile, []*provisioner.RemoteFile, map[string]interface{}, error) {
	files := []api.CustomFile{}
	manifests := []*provisioner.RemoteFile{}
	configsetFiles := make(map[string]interface{})

	for _, m := range mspecs {
		rendered, ma, err := regularOrConfigSetFile(m.RemoteFileSpec, r)
		if err != nil {
			return files, manifests, configsetFiles, nil
		}
		var name string
		if m.Name == "" {
			if m.RemoteFileSpec.Source.Path == "" {
				return files, manifests, configsetFiles, fmt.Errorf("manifest.name is required in %v", m)
			}
			name = filepath.Base(m.RemoteFileSpec.Source.Path)
		} else {
			name = m.Name
		}

		remotePath := filepath.Join("/srv/kube-aws/plugins", pluginName, name)
		if rendered == nil {
			configsetFiles[remotePath] = map[string]interface{}{
				"content": ma,
			}
			manifests = append(manifests, provisioner.NewRemoteFileAtPath(remotePath, []byte{}))
			continue
		}

		f := api.CustomFile{
			Path:        remotePath,
			Permissions: 0644,
			Content:     *rendered,
		}
		files = append(files, f)
		manifests = append(manifests, provisioner.NewRemoteFileAtPath(f.Path, []byte(f.Content)))
	}

	return files, manifests, configsetFiles, nil
}

func renderHelmReleases(pluginName string, releases api.HelmReleases) ([]api.HelmReleaseFileset, error) {
	releaseFileSets := []api.HelmReleaseFileset{}

	for _, releaseConfig := range releases {
		valuesFilePath := filepath.Join("/srv/kube-aws/plugins", pluginName, "helm", "releases", releaseConfig.Name, "values.yaml")
		valuesFileContent, err := json.Marshal(releaseConfig.Values)
		if err != nil {
			return releaseFileSets, fmt.Errorf("Unexpected error in HelmReleasePlugin: %v", err)
		}
		releaseFileData := map[string]interface{}{
			"values": map[string]string{
				"file": valuesFilePath,
			},
			"chart": map[string]string{
				"name":    releaseConfig.Name,
				"version": releaseConfig.Version,
			},
		}
		releaseFilePath := filepath.Join("/srv/kube-aws/plugins", pluginName, "helm", "releases", releaseConfig.Name, "release.json")
		releaseFileContent, err := json.Marshal(releaseFileData)
		if err != nil {
			return releaseFileSets, fmt.Errorf("Unexpected error in HelmReleasePlugin: %v", err)
		}
		r := api.HelmReleaseFileset{
			ValuesFile: provisioner.NewRemoteFileAtPath(
				valuesFilePath,
				valuesFileContent,
			),
			ReleaseFile: provisioner.NewRemoteFileAtPath(
				releaseFilePath,
				releaseFileContent,
			),
		}
		releaseFileSets = append(releaseFileSets, r)
	}
	return releaseFileSets, nil
}

// getFlags - generic loader of command line flags, returns a slice of flags
func getFlags(render *plugincontents.TemplateRenderer, flags api.CommandLineFlags) ([]api.CommandLineFlag, error) {
	extraFlags := []api.CommandLineFlag{}

	for _, f := range flags {
		v, err := render.String(f.Value)
		if err != nil {
			return extraFlags, fmt.Errorf("failed to load apisersver flags: %v", err)
		}
		newFlag := api.CommandLineFlag{
			Name:  f.Name,
			Value: v,
		}
		extraFlags = append(extraFlags, newFlag)
	}
	return extraFlags, nil
}

func (e ClusterExtension) Controller(clusterConfig interface{}) (*controller, error) {
	logger.Debugf("ClusterExtension.Controller(): Generating Plugin Controller user-data extras")
	apiServerFlags := api.CommandLineFlags{}
	apiServerVolumes := api.APIServerVolumes{}
	controllerFlags := api.CommandLineFlags{}
	kubeProxyConfig := map[string]interface{}{}
	kubeletFlags := api.CommandLineFlags{}
	kubeSchedulerFlags := api.CommandLineFlags{}

	systemdUnits := []api.CustomSystemdUnit{}
	files := []api.CustomFile{}
	iamStatements := api.IAMPolicyStatements{}
	nodeLabels := api.NodeLabels{}
	configsets := map[string]interface{}{}
	archivedFiles := []provisioner.RemoteFileSpec{}
	var kubeconfig string
	kubeletMounts := []api.ContainerVolumeMount{}
	manifests := []*provisioner.RemoteFile{}
	releaseFilesets := []api.HelmReleaseFileset{}

	for _, p := range e.plugins {
		//fmt.Fprintf(os.Stderr, "plugin=%+v configs=%+v", p, e.configs)
		if enabled, pc := p.EnabledIn(e.Configs); enabled {
			values, err := pluginutil.MergeValues(p.Spec.Cluster.Values, pc.Values)
			if err != nil {
				return nil, err
			}
			render := plugincontents.NewTemplateRenderer(p, values, clusterConfig)

			extraApiServerFlags, err := getFlags(render, p.Spec.Cluster.Kubernetes.APIServer.Flags)
			if err != nil {
				return nil, err
			}
			apiServerFlags = append(apiServerFlags, extraApiServerFlags...)

			extraControllerManagerFlags, err := getFlags(render, p.Spec.Cluster.Kubernetes.ControllerManager.Flags)
			if err != nil {
				return nil, err
			}
			controllerFlags = append(controllerFlags, extraControllerManagerFlags...)

			extraKubeSchedulerFlags, err := getFlags(render, p.Spec.Cluster.Kubernetes.KubeScheduler.Flags)
			if err != nil {
				return nil, err
			}
			kubeSchedulerFlags = append(kubeSchedulerFlags, extraKubeSchedulerFlags...)

			extraKubeletFlags, err := getFlags(render, p.Spec.Cluster.Kubernetes.Kubelet.Flags)
			if err != nil {
				return nil, err
			}
			kubeletFlags = append(kubeletFlags, extraKubeletFlags...)

			for key, value := range p.Spec.Cluster.Kubernetes.KubeProxy.Config {
				kubeProxyConfig[key] = value
			}

			apiServerVolumes = append(apiServerVolumes, p.Spec.Cluster.Kubernetes.APIServer.Volumes...)

			extraUnits, err := renderMachineSystemdUnits(render, p.Spec.Cluster.Machine.Roles.Controller.Systemd.Units)
			if err != nil {
				return nil, fmt.Errorf("failed adding systemd units to etcd: %v", err)
			}
			systemdUnits = append(systemdUnits, extraUnits...)

			extraArchivedFiles, extraFiles, extraConfigSetFiles, err := renderMachineFilesAndConfigSets(render, p.Spec.Cluster.Roles.Controller.Files)
			if err != nil {
				return nil, fmt.Errorf("failed adding files to controller: %v", err)
			}
			archivedFiles = append(archivedFiles, extraArchivedFiles...)
			files = append(files, extraFiles...)

			iamStatements = append(iamStatements, p.Spec.Cluster.Machine.Roles.Controller.IAM.Policy.Statements...)

			for k, v := range p.Spec.Cluster.Machine.Roles.Controller.Kubelet.NodeLabels {
				nodeLabels[k] = v
			}

			if p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Kubeconfig != "" {
				kubeconfig = p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Kubeconfig
			}

			if len(p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Mounts) > 0 {
				kubeletMounts = append(kubeletMounts, p.Spec.Cluster.Machine.Roles.Controller.Kubelet.Mounts...)
			}

			extraFiles, extraManifests, manifestConfigSetFiles, err := renderKubernetesManifests(p.Name, render, p.Spec.Cluster.Kubernetes.Manifests)
			if err != nil {
				return nil, fmt.Errorf("failed adding kubernetes manifests to controller: %v", err)
			}
			files = append(files, extraFiles...)
			manifests = append(manifests, extraManifests...)
			// merge the manifest configsets into machine generated configsetfiles
			for k, v := range manifestConfigSetFiles {
				extraConfigSetFiles[k] = v
			}
			configsets[p.Name] = map[string]map[string]interface{}{
				"files": extraConfigSetFiles,
			}

			extraReleaseFileSets, err := renderHelmReleases(p.Name, p.Spec.Cluster.Helm.Releases)
			releaseFilesets = append(releaseFilesets, extraReleaseFileSets...)
		}
	}

	return &controller{
		ArchivedFiles:           archivedFiles,
		APIServerFlags:          apiServerFlags,
		ControllerFlags:         controllerFlags,
		KubeSchedulerFlags:      kubeSchedulerFlags,
		KubeProxyConfig:         kubeProxyConfig,
		KubeletFlags:            kubeletFlags,
		APIServerVolumes:        apiServerVolumes,
		Files:                   files,
		SystemdUnits:            systemdUnits,
		IAMPolicyStatements:     iamStatements,
		NodeLabels:              nodeLabels,
		KubeletVolumeMounts:     kubeletMounts,
		Kubeconfig:              kubeconfig,
		CfnInitConfigSets:       configsets,
		KubernetesManifestFiles: manifests,
		HelmReleaseFilesets:     releaseFilesets,
	}, nil
}

func (e ClusterExtension) Etcd(clusterConfig interface{}) (*etcd, error) {
	logger.Debugf("ClusterExtension.Etcd(): Generating Plugin Etcd user-data extras")
	systemdUnits := []api.CustomSystemdUnit{}
	files := []api.CustomFile{}
	iamStatements := api.IAMPolicyStatements{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.Configs); enabled {
			values, err := pluginutil.MergeValues(p.Spec.Cluster.Values, pc.Values)
			if err != nil {
				return nil, err
			}
			render := plugincontents.NewTemplateRenderer(p, values, clusterConfig)

			extraUnits, err := renderMachineSystemdUnits(render, p.Spec.Cluster.Machine.Roles.Etcd.Systemd.Units)
			if err != nil {
				return nil, fmt.Errorf("failed adding systemd units to etcd: %v", err)
			}
			systemdUnits = append(systemdUnits, extraUnits...)

			extraFiles, err := simpleRenderMachineFiles(render, p.Spec.Cluster.Roles.Etcd.Files)
			if err != nil {
				return nil, fmt.Errorf("failed adding files to etcd: %v", err)
			}
			files = append(files, extraFiles...)

			iamStatements = append(iamStatements, p.Spec.Cluster.Roles.Etcd.IAM.Policy.Statements...)
		}
	}

	return &etcd{
		Files:               files,
		SystemdUnits:        systemdUnits,
		IAMPolicyStatements: iamStatements,
	}, nil
}
