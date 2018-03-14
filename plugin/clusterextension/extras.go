package clusterextension

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/plugin/plugincontents"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginutil"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginvalue"
)

type ClusterExtension struct {
	plugins []*pluginmodel.Plugin
	configs model.PluginConfigs
}

func NewExtrasFromPlugins(plugins []*pluginmodel.Plugin, configs model.PluginConfigs) ClusterExtension {
	return ClusterExtension{
		plugins: plugins,
		configs: configs,
	}
}

type stack struct {
	Resources map[string]interface{}
}

func (e ClusterExtension) RootStack() (*stack, error) {
	resources := map[string]interface{}{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.configs); enabled {
			values := pluginutil.MergeValues(p.Spec.Values, pc.Values)

			render := plugincontents.TemplateRendererFor(p, values)

			{
				m, err := render.MapFromContents(p.Spec.CloudFormation.Stacks.Root.Resources.Append.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load additional resources for root stack: %v", err)
				}
				for k, v := range m {
					resources[k] = v
				}
			}
		}
	}

	return &stack{
		Resources: resources,
	}, nil
}

type worker struct {
	Files               []model.CustomFile
	SystemdUnits        []model.CustomSystemdUnit
	IAMPolicyStatements []model.IAMPolicyStatement
	NodeLabels          model.NodeLabels
	FeatureGates        model.FeatureGates
}

type controller struct {
	APIServerFlags      pluginmodel.APIServerFlags
	APIServerVolumes    pluginmodel.APIServerVolumes
	Files               []model.CustomFile
	SystemdUnits        []model.CustomSystemdUnit
	IAMPolicyStatements []model.IAMPolicyStatement
	NodeLabels          model.NodeLabels
}

type etcd struct {
	Files               []model.CustomFile
	SystemdUnits        []model.CustomSystemdUnit
	IAMPolicyStatements []model.IAMPolicyStatement
}

func (e ClusterExtension) NodePoolStack() (*stack, error) {
	resources := map[string]interface{}{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.configs); enabled {
			values := pluginutil.MergeValues(p.Spec.Values, pc.Values)
			render := plugincontents.TemplateRendererFor(p, values)

			m, err := render.MapFromContents(p.Spec.CloudFormation.Stacks.NodePool.Resources.Append.Contents)
			if err != nil {
				return nil, fmt.Errorf("failed to load additioanl resources for worker node-pool stack: %v", err)
			}
			for k, v := range m {
				resources[k] = v
			}
		}
	}
	return &stack{
		Resources: resources,
	}, nil
}

func (e ClusterExtension) Worker() (*worker, error) {
	files := []model.CustomFile{}
	systemdUnits := []model.CustomSystemdUnit{}
	iamStatements := []model.IAMPolicyStatement{}
	nodeLabels := model.NodeLabels{}
	featureGates := model.FeatureGates{}

	for _, p := range e.plugins {
		if enabled, _ := p.EnabledIn(e.configs); enabled {
			load := plugincontents.LoaderFor(p)

			for _, d := range p.Spec.Node.Roles.Worker.Systemd.Units {
				u := model.CustomSystemdUnit{
					Name:    d.Name,
					Command: "start",
					Content: d.Contents.Inline,
					Enable:  true,
					Runtime: false,
				}
				systemdUnits = append(systemdUnits, u)
			}

			for _, d := range p.Spec.Node.Roles.Worker.Storage.Files {
				s, err := load.StringFrom(d.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load plugin worker file contents: %v", err)
				}
				f := model.CustomFile{
					Path:        d.Path,
					Permissions: d.Permissions,
					Content:     s,
				}
				files = append(files, f)
			}

			iamStatements = append(iamStatements, p.Spec.Node.Roles.Worker.IAM.Policy.Statements...)

			for k, v := range p.Spec.Node.Roles.Worker.Kubelet.NodeLabels {
				nodeLabels[k] = v
			}

			for k, v := range p.Spec.Node.Roles.Worker.Kubelet.FeatureGates {
				featureGates[k] = v
			}
		}
	}

	return &worker{
		Files:               files,
		SystemdUnits:        systemdUnits,
		IAMPolicyStatements: iamStatements,
		NodeLabels:          nodeLabels,
		FeatureGates:        featureGates,
	}, nil
}

func (e ClusterExtension) ControlPlaneStack() (*stack, error) {
	resources := map[string]interface{}{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.configs); enabled {
			values := pluginutil.MergeValues(p.Spec.Values, pc.Values)

			render := plugincontents.TemplateRendererFor(p, values)

			{
				m, err := render.MapFromContents(p.Spec.CloudFormation.Stacks.ControlPlane.Resources.Append.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load additional resources for control-plane stack: %v", err)
				}
				for k, v := range m {
					resources[k] = v
				}
			}
		}
	}

	return &stack{
		Resources: resources,
	}, nil
}

func (e ClusterExtension) Controller() (*controller, error) {
	apiServerFlags := pluginmodel.APIServerFlags{}
	apiServerVolumes := pluginmodel.APIServerVolumes{}
	systemdUnits := []model.CustomSystemdUnit{}
	files := []model.CustomFile{}
	iamStatements := model.IAMPolicyStatements{}
	nodeLabels := model.NodeLabels{}

	for _, p := range e.plugins {
		if enabled, pc := p.EnabledIn(e.configs); enabled {
			values := pluginutil.MergeValues(p.Spec.Values, pc.Values)

			load := plugincontents.LoaderFor(p)

			{
				render := pluginvalue.TemplateRendererFor(p, values)
				for _, f := range p.Spec.Kubernetes.APIServer.Flags {
					v, err := render.StringFrom(f.Value)
					if err != nil {
						return nil, fmt.Errorf("failed to load apisersver flags: %v", err)
					}
					newFlag := pluginmodel.APIServerFlag{
						Name:  f.Name,
						Value: v,
					}
					apiServerFlags = append(apiServerFlags, newFlag)
				}
			}

			apiServerVolumes = append(apiServerVolumes, p.Spec.Kubernetes.APIServer.Volumes...)

			for _, d := range p.Spec.Node.Roles.Controller.Systemd.Units {
				u := model.CustomSystemdUnit{
					Name:    d.Name,
					Command: "start",
					Content: d.Contents.Inline,
					Enable:  true,
					Runtime: false,
				}
				systemdUnits = append(systemdUnits, u)
			}

			for _, d := range p.Spec.Node.Roles.Controller.Storage.Files {
				s, err := load.StringFrom(d.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load plugin controller file contents: %v", err)
				}
				f := model.CustomFile{
					Path:        d.Path,
					Permissions: d.Permissions,
					Content:     s,
				}
				files = append(files, f)
			}

			iamStatements = append(iamStatements, p.Spec.Node.Roles.Controller.IAM.Policy.Statements...)

			for k, v := range p.Spec.Node.Roles.Controller.Kubelet.NodeLabels {
				nodeLabels[k] = v
			}
		}
	}

	return &controller{
		APIServerFlags:      apiServerFlags,
		APIServerVolumes:    apiServerVolumes,
		Files:               files,
		SystemdUnits:        systemdUnits,
		IAMPolicyStatements: iamStatements,
		NodeLabels:          nodeLabels,
	}, nil
}

func (e ClusterExtension) Etcd() (*etcd, error) {
	systemdUnits := []model.CustomSystemdUnit{}
	files := []model.CustomFile{}
	iamStatements := model.IAMPolicyStatements{}

	for _, p := range e.plugins {
		if enabled, _ := p.EnabledIn(e.configs); enabled {
			load := plugincontents.LoaderFor(p)

			for _, d := range p.Spec.Node.Roles.Etcd.Systemd.Units {
				u := model.CustomSystemdUnit{
					Name:    d.Name,
					Command: "start",
					Content: d.Contents.Inline,
					Enable:  true,
					Runtime: false,
				}
				systemdUnits = append(systemdUnits, u)
			}

			for _, d := range p.Spec.Node.Roles.Etcd.Storage.Files {
				s, err := load.StringFrom(d.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load plugin etcd file contents: %v", err)
				}
				f := model.CustomFile{
					Path:        d.Path,
					Permissions: d.Permissions,
					Content:     s,
				}
				files = append(files, f)
			}

			iamStatements = append(iamStatements, p.Spec.Node.Roles.Etcd.IAM.Policy.Statements...)
		}
	}

	return &etcd{
		Files:               files,
		SystemdUnits:        systemdUnits,
		IAMPolicyStatements: iamStatements,
	}, nil
}
