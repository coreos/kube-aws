package model

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
)

func newStack(stackName string, conf *Config, opts api.StackTemplateOptions, assetsConfig *credential.CompactAssets, tmplCtx func(stack *Stack) (interface{}, error), init func(stack *Stack) error) (*Stack, error) {
	logger.Debugf("Called newStack")

	stack := &Stack{
		StackName:            stackName,
		StackTemplateOptions: opts,
		ClusterName:          conf.ClusterName,
		S3URI:                conf.S3URI,
		Region:               conf.Region,
		AssetsConfig:         assetsConfig,
		Config:               conf,
	}

	ctx, err := tmplCtx(stack)
	if err != nil {
		return nil, err
	}
	stack.tmplCtx = ctx

	if err := init(stack); err != nil {
		return nil, err
	}

	assets, err := stack.buildAssets()
	if err != nil {
		return nil, err
	}
	stack.assets = assets

	return stack, nil
}

// NewControlPlaneStack reads the specified cluster spec along with all the referenced files into memory.
// Any configuration error like a reference to a missing file results in kube-aws existing with an error.
func NewControlPlaneStack(conf *Config, opts api.StackTemplateOptions, extras clusterextension.ClusterExtension, assetsConfig *credential.CompactAssets) (*Stack, error) {
	return newStack(
		conf.ControlPlaneStackName(),
		conf,
		opts,
		assetsConfig,
		func(stack *Stack) (interface{}, error) {
			// Import all the managed subnets from the network stack
			subnets, err := stack.Config.Subnets.ImportFromNetworkStackRetainingNames()
			if err != nil {
				return nil, fmt.Errorf("failed to import subnets from network stack: %v", err)
			}
			vpc := stack.Config.VPC.ImportFromNetworkStack()

			return ControllerTmplCtx{
				Stack:   stack,
				Config:  conf,
				VPC:     vpc,
				Subnets: subnets,
			}, nil
		},
		func(stack *Stack) error {
			extraStack, err := extras.ControlPlaneStack(stack)
			if err != nil {
				return fmt.Errorf("failed to load control-plane stack extras from plugins: %v", err)
			}
			stack.ExtraCfnResources = extraStack.Resources
			stack.ExtraCfnTags = extraStack.Tags
			stack.ExtraCfnOutputs = extraStack.Outputs

			extraController, err := extras.Controller(conf)
			if err != nil {
				return fmt.Errorf("failed to load controller node extras from plugins: %v", err)
			}

			if len(conf.Kubelet.Kubeconfig) == 0 {
				conf.Kubelet.Kubeconfig = extraController.Kubeconfig
			}
			conf.Kubelet.Mounts = append(conf.Kubelet.Mounts, extraController.KubeletVolumeMounts...)
			conf.APIServerFlags = append(conf.APIServerFlags, extraController.APIServerFlags...)
			conf.ControllerFlags = append(conf.ControllerFlags, extraController.ControllerFlags...)
			conf.APIServerVolumes = append(conf.APIServerVolumes, extraController.APIServerVolumes...)
			conf.Controller.CustomSystemdUnits = append(conf.Controller.CustomSystemdUnits, extraController.SystemdUnits...)
			conf.Controller.CustomFiles = append(conf.Controller.CustomFiles, extraController.Files...)
			conf.Controller.IAMConfig.Policy.Statements = append(conf.Controller.IAMConfig.Policy.Statements, extraController.IAMPolicyStatements...)
			conf.KubeAWSVersion = VERSION
			for k, v := range extraController.NodeLabels {
				conf.Controller.NodeLabels[k] = v
			}
			conf.HelmReleaseFilesets = extraController.HelmReleaseFilesets
			conf.KubernetesManifestFiles = extraController.KubernetesManifestFiles

			if len(conf.StackTags) == 0 {
				conf.StackTags = make(map[string]string, 1)
			}
			conf.StackTags["kube-aws:version"] = VERSION

			stack.archivedFiles = extraController.ArchivedFiles
			stack.CfnInitConfigSets = extraController.CfnInitConfigSets

			return stack.RenderAddControllerUserdata(opts)
		},
	)
}

func NewNetworkStack(conf *Config, nodePools []*Stack, opts api.StackTemplateOptions, extras clusterextension.ClusterExtension, assetsConfig *credential.CompactAssets) (*Stack, error) {
	return newStack(
		conf.NetworkStackName(),
		conf,
		opts,
		assetsConfig,
		func(stack *Stack) (interface{}, error) {
			nps := []WorkerTmplCtx{}
			for _, s := range nodePools {
				nps = append(nps, s.tmplCtx.(WorkerTmplCtx))
			}

			return NetworkTmplCtx{
				Stack:           stack,
				Config:          conf,
				WorkerNodePools: nps,
			}, nil
		},
		func(stack *Stack) error {
			extraStack, err := extras.NetworkStack(stack)
			if err != nil {
				return fmt.Errorf("failed to load network stack extras from plugins: %v", err)
			}
			stack.ExtraCfnResources = extraStack.Resources
			return nil
		},
	)
}

func NewEtcdStack(conf *Config, opts api.StackTemplateOptions, extras clusterextension.ClusterExtension, assetsConfig *credential.CompactAssets, s *Context) (*Stack, error) {
	return newStack(
		conf.EtcdStackName(),
		conf,
		opts,
		assetsConfig,
		func(stack *Stack) (interface{}, error) {
			// create the context that will be used to build the assets (combination of config + existing state)
			existingState, err := s.InspectEtcdExistingState(conf)
			if err != nil {
				return nil, fmt.Errorf("Could not inspect existing etcd state: %v", err)
			}

			// Import all the managed subnets from the network stack
			subnets, err := stack.Config.Subnets.ImportFromNetworkStackRetainingNames()
			if err != nil {
				return nil, fmt.Errorf("failed to import subnets from network stack: %v", err)
			}

			nodes := []EtcdNode{}
			for i, n := range stack.Config.EtcdNodes {
				n2 := n
				n2.subnet = subnets[i%len(subnets)]
				nodes = append(nodes, n2)
			}

			return EtcdTmplCtx{
				Stack:             stack,
				Config:            conf,
				EtcdExistingState: existingState,
				EtcdNodes:         nodes,
			}, nil
		}, func(stack *Stack) error {
			extraStack, err := extras.EtcdStack(stack)
			if err != nil {
				return fmt.Errorf("failed to load etcd stack extras from plugins: %v", err)
			}
			stack.ExtraCfnResources = extraStack.Resources
			stack.ExtraCfnTags = extraStack.Tags
			stack.ExtraCfnOutputs = extraStack.Outputs

			extraEtcd, err := extras.Etcd()
			if err != nil {
				return fmt.Errorf("failed to load etcd node extras from plugins: %v", err)
			}

			conf.Etcd.CustomSystemdUnits = append(conf.Etcd.CustomSystemdUnits, extraEtcd.SystemdUnits...)
			conf.Etcd.CustomFiles = append(conf.Etcd.CustomFiles, extraEtcd.Files...)
			conf.Etcd.IAMConfig.Policy.Statements = append(conf.Etcd.IAMConfig.Policy.Statements, extraEtcd.IAMPolicyStatements...)
			// TODO Not implemented yet
			//stack.archivedFiles = extraEtcd.ArchivedFiles
			//stack.CfnInitConfigSets = extraEtcd.CfnInitConfigSets

			return stack.RenderAddEtcdUserdata(opts)
		},
	)
}

func NewWorkerStack(conf *Config, npconf *NodePoolConfig, opts api.StackTemplateOptions, extras clusterextension.ClusterExtension, assetsConfig *credential.CompactAssets) (*Stack, error) {
	return newStack(
		npconf.StackName(),
		conf,
		opts,
		assetsConfig,
		func(stack *Stack) (interface{}, error) {
			return WorkerTmplCtx{
				Stack:          stack,
				NodePoolConfig: npconf,
			}, nil
		},
		func(stack *Stack) error {
			stack.NodePoolConfig = npconf

			extraStack, err := extras.NodePoolStack(conf)
			if err != nil {
				return fmt.Errorf("failed to load node pool stack extras from plugins: %v", err)
			}
			stack.ExtraCfnResources = extraStack.Resources
			stack.ExtraCfnTags = extraStack.Tags
			stack.ExtraCfnOutputs = extraStack.Outputs

			extraWorker, err := extras.Worker(conf)
			if err != nil {
				return fmt.Errorf("failed to load worker node extras from plugins: %v", err)
			}
			if len(npconf.Kubelet.Kubeconfig) == 0 {
				npconf.Kubelet.Kubeconfig = extraWorker.Kubeconfig
			}
			npconf.Kubelet.Mounts = append(conf.Kubelet.Mounts, extraWorker.KubeletVolumeMounts...)
			npconf.CustomSystemdUnits = append(npconf.CustomSystemdUnits, extraWorker.SystemdUnits...)
			npconf.CustomFiles = append(npconf.CustomFiles, extraWorker.Files...)
			npconf.IAMConfig.Policy.Statements = append(npconf.IAMConfig.Policy.Statements, extraWorker.IAMPolicyStatements...)
			npconf.KubeAWSVersion = VERSION
			if len(npconf.StackTags) == 0 {
				npconf.StackTags = make(map[string]string, 1)
			}
			npconf.StackTags["kube-aws:version"] = VERSION

			for k, v := range extraWorker.NodeLabels {
				npconf.NodeSettings.NodeLabels[k] = v
			}
			for k, v := range extraWorker.FeatureGates {
				npconf.NodeSettings.FeatureGates[k] = v
			}

			stack.archivedFiles = extraWorker.ArchivedFiles
			stack.CfnInitConfigSets = extraWorker.CfnInitConfigSets

			return stack.RenderAddWorkerUserdata(opts)
		},
	)
}
