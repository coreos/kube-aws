package config

import "github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"

func (c *Config) MapLegacySettings() error {
	// Process/Map legacy Experimental settings into plugin friendly data structures....
	if c.Cluster.KubeProxy.IPVSMode.Enabled {
		c.Cluster.Kubernetes.FeatureGates["SupportIPVSProxyMode"] = true
	}
	if c.Cluster.Experimental.Admission.Priority.Enabled {
		c.Cluster.Kubernetes.FeatureGates["PodPriority"] = true
	}
	if c.Cluster.Experimental.Admission.PersistentVolumeClaimResize.Enabled {
		c.Cluster.Kubernetes.FeatureGates["ExpandPersistentVolumes"] = true
		c.Cluster.Kubernetes.AdmissionControllers["PersistentVolumeClaimResize"] = 160
	}

	// handle all of the possible admission controllers so that we can just render out the line using the model
	// instead of relaying on complex templating...
	if c.Cluster.Experimental.Admission.PodSecurityPolicy.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["PodSecurityPolicy"] = 60
	}
	if c.Cluster.Experimental.Admission.AlwaysPullImages.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["AlwaysPullImages"] = 70
	}
	if c.Cluster.Experimental.NodeAuthorizer.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["NodeRestriction"] = 80
	}
	if c.Cluster.Experimental.Admission.DenyEscalatingExec.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["DenyEscalatingExec"] = 100
	}
	if c.Cluster.Experimental.Admission.Initializers.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["Initializers"] = 110
	}
	if c.Cluster.Experimental.Admission.Priority.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["Priority"] = 120
	}
	if c.Cluster.Experimental.Admission.MutatingAdmissionWebhook.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["MutatingAdmissionWebhook"] = 140
	}
	if c.Cluster.Experimental.Admission.ValidatingAdmissionWebhook.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["MutatingAdmissionWebhook"] = 150
	}
	if c.Cluster.Experimental.Admission.PersistentVolumeClaimResize.Enabled {
		c.Cluster.Kubernetes.AdmissionControllers["MutatingAdmissionWebhook"] = 150
	}

	// PLEASE NOTE - I think we should move APIServerFlags into 'model' and then plugins should use it from model.
	// Also I know that most of these features could be migrated out into core plugins -
	// this is just a demonstration of moving the legacy settings from the rendering/template logic into models
	// so that they can also be manipulated by plugins (i.e. a migration step)
	if c.Cluster.Experimental.Oidc.Enabled {
		c.APIServerFlags = append(c.APIServerFlags, pluginmodel.APIServerFlag{Name: "oidc-issuer-url", Value: c.Cluster.Experimental.Oidc.IssuerUrl},
			pluginmodel.APIServerFlag{Name: "oidc-client-id", Value: c.Cluster.Experimental.Oidc.ClientId})
		if c.Cluster.Experimental.Oidc.UsernameClaim != "" {
			c.APIServerFlags = append(c.APIServerFlags, pluginmodel.APIServerFlag{Name: "oidc-username-claim", Value: c.Cluster.Experimental.Oidc.UsernameClaim})
		}
		if c.Cluster.Experimental.Oidc.GroupsClaim != "" {
			c.APIServerFlags = append(c.APIServerFlags, pluginmodel.APIServerFlag{Name: "oidc-groups-claim", Value: c.Cluster.Experimental.Oidc.GroupsClaim})
		}
	}

	if c.Cluster.Addons.APIServerAggregator.Enabled {
		c.APIServerFlags = append(c.APIServerFlags,
			pluginmodel.APIServerFlag{Name: "requestheader-client-ca-file", Value: "/etc/kubernetes/ssl/ca.pem"},
			pluginmodel.APIServerFlag{Name: "requestheader-allowed-names", Value: "aggregator"},
			pluginmodel.APIServerFlag{Name: "requestheader-extra-headers-prefix", Value: "X-Remote-Extra-"},
			pluginmodel.APIServerFlag{Name: "requestheader-group-headers", Value: "X-Remote-Group"},
			pluginmodel.APIServerFlag{Name: "requestheader-username-headers", Value: "X-Remote-User"},
			pluginmodel.APIServerFlag{Name: "enable-aggregator-routing", Value: "false"},
			pluginmodel.APIServerFlag{Name: "proxy-client-cert-file", Value: "/etc/kubernetes/ssl/apiserver-aggregator.pem"},
			pluginmodel.APIServerFlag{Name: "proxy-client-key-fil", Value: "/etc/kubernetes/ssl/apiserver-aggregator-key.pem"})
	}

	return nil
}
