package integration

import (
	"os"
	"strings"
	"testing"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
)

func TestPlugin(t *testing.T) {
	kubeAwsSettings := newKubeAwsSettingsFromEnv(t)

	s3URI, s3URIExists := os.LookupEnv("KUBE_AWS_S3_DIR_URI")

	if !s3URIExists || s3URI == "" {
		s3URI = "s3://examplebucket/exampledir"
		t.Logf(`Falling back s3URI to a stub value "%s" for tests of validating stack templates. No assets will actually be uploaded to S3`, s3URI)
	}

	mainClusterYaml := kubeAwsSettings.mainClusterYaml()
	minimalValidConfigYaml := mainClusterYaml + `
availabilityZone: us-west-1c
`
	validCases := []struct {
		context       string
		clusterYaml   string
		plugins       []helper.TestPlugin
		assertConfig  []ConfigTester
		assertCluster []ClusterTester
	}{
		{
			context: "WithAddons",
			clusterYaml: minimalValidConfigYaml + `
kubeAwsPlugins:
  myPlugin:
    enabled: true
    queue:
      nqme: baz1

worker:
  nodePools:
  - name: pool1
    kubeAwsPlugins:
      myPlugin:
        enabled: true
        queue:
          name: baz2
`,
			plugins: []helper.TestPlugin{
				helper.TestPlugin{
					Name: "my-plugin",
					Yaml: `
metadata:
  name: my-plugin
  version: 0.0.1
spec:
  configuration:
    values:
      queue:
        name: bar
    cloudformation:
      stacks:
        controlPlane:
          resources:
            append:
              inline: |
                {
                  "QueueFromMyPlugin": {
                    "Type": "AWS::SQS::Queue",
                    "Properties": {
                      "QueueName": {{quote .Values.queue.name}}
                    }
                  }
                }
        nodePool:
          resources:
            append:
              inline: |
                {
                  "QueueFromMyPlugin": {
                    "Type": "AWS::SQS::Queue",
                    "Properties": {
                      "QueueName": {{quote .Values.queue.name}}
                    }
                  }
                }
        root:
          resources:
            append:
              inline: |
                {
                  "QueueFromMyPlugin": {
                    "Type": "AWS::SQS::Queue",
                    "Properties": {
                    "QueueName": {{quote .Values.queue.name}}
                    }
                  }
                }
    node:
      roles:
        controller:
          kubelet:
            nodeLabels:
              role: controller
          systemd:
            units:
            - name: save-queue-name.service
              contents:
                inline: |
                  [Unit]
        etcd:
          systemd:
            units:
            - name: save-queue-name.service
              contents:
                inline: |
                  [Unit]
        worker:
          kubelet:
            nodeLabels:
              role: worker
            featureGates:
              Accelerators: "true"
          systemd:
            units:
            - name: save-queue-name.service
              contents:
                inline: |
                  [Unit]
`,
				},
			},
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					cp := c.Plugins["myPlugin"]

					if !cp.Enabled {
						t.Errorf("The plugin should have been enabled: %+v", cp)
					}

					if q, ok := cp.Settings["queue"].(map[string]interface{}); ok {
						if m, ok := q["name"].(string); ok {
							if m != "baz1" {
								t.Errorf("The plugin should have queue.name set to \"baz1\", but was set to \"%s\"", m)
							}
						}
					}

					np := c.NodePools[0].Plugins["myPlugin"]

					if !np.Enabled {
						t.Errorf("The plugin should have been enabled: %+v", np)
					}

					if q, ok := np.Settings["queue"].(map[string]interface{}); ok {
						if m, ok := q["name"].(string); ok {
							if m != "baz2" {
								t.Errorf("The plugin should have queue.name set to \"baz2\", but was set to \"%s\"", m)
							}
						}
					}
				},
			},
			assertCluster: []ClusterTester{
				func(c root.Cluster, t *testing.T) {
					cp := c.ControlPlane()
					np := c.NodePools()[0]

					// A kube-aws plugin can inject systemd units
					controllerUserdataS3Part := cp.UserDataController.Parts[model.USERDATA_S3].Asset.Content
					if !strings.Contains(controllerUserdataS3Part, "save-queue-name.service") {
						t.Errorf("Invalid controller userdata: %v", controllerUserdataS3Part)
					}

					etcdUserdataS3Part := cp.UserDataEtcd.Parts[model.USERDATA_S3].Asset.Content
					if !strings.Contains(etcdUserdataS3Part, "save-queue-name.service") {
						t.Errorf("Invalid etcd userdata: %v", etcdUserdataS3Part)
					}

					workerUserdataS3Part := np.UserDataWorker.Parts[model.USERDATA_S3].Asset.Content
					if !strings.Contains(workerUserdataS3Part, "save-queue-name.service") {
						t.Errorf("Invalid worker userdata: %v", workerUserdataS3Part)
					}

					// A kube-aws plugin can inject custom cfn stack resources
					controlPlaneStackTemplate, err := cp.RenderStackTemplateAsString()
					if err != nil {
						t.Errorf("failed to render control-plane stack template: %v", err)
					}
					if !strings.Contains(controlPlaneStackTemplate, "QueueFromMyPlugin") {
						t.Errorf("Invalid control-plane stack template: %v", controlPlaneStackTemplate)
					}

					rootStackTemplate, err := c.RenderStackTemplateAsString()
					if err != nil {
						t.Errorf("failed to render root stack template: %v", err)
					}
					if !strings.Contains(rootStackTemplate, "QueueFromMyPlugin") {
						t.Errorf("Invalid root stack template: %v", rootStackTemplate)
					}

					nodePoolStackTemplate, err := np.RenderStackTemplateAsString()
					if err != nil {
						t.Errorf("failed to render worker node pool stack template: %v", err)
					}
					if !strings.Contains(nodePoolStackTemplate, "QueueFromMyPlugin") {
						t.Errorf("Invalid worker node pool stack template: %v", nodePoolStackTemplate)
					}

					// A kube-aws plugin can inject node labels
					if !strings.Contains(controllerUserdataS3Part, "role=controller") {
						t.Error("missing controller node label: role=controller")
					}

					if !strings.Contains(workerUserdataS3Part, "role=worker") {
						t.Error("missing worker node label: role=worker")
					}

					// A kube-aws plugin can activate feature gates
					if !strings.Contains(workerUserdataS3Part, `--feature-gates="Accelerators=true"`) {
						t.Error("missing worker feature gate: Accelerators=true")
					}
				},
			},
		},
	}

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.clusterYaml
			providedConfig, err := config.ConfigFromBytesWithEncryptService([]byte(configBytes), helper.DummyEncryptService{})
			if err != nil {
				t.Errorf("failed to parse config %s: %v", configBytes, err)
				t.FailNow()
			}

			t.Run("AssertConfig", func(t *testing.T) {
				for _, assertion := range validCase.assertConfig {
					assertion(providedConfig, t)
				}
			})

			helper.WithDummyCredentials(func(dummyAssetsDir string) {
				var stackTemplateOptions = root.NewOptions(s3URI, false, false)
				stackTemplateOptions.AssetsDir = dummyAssetsDir
				stackTemplateOptions.ControllerTmplFile = "../../core/controlplane/config/templates/cloud-config-controller"
				stackTemplateOptions.WorkerTmplFile = "../../core/controlplane/config/templates/cloud-config-worker"
				stackTemplateOptions.EtcdTmplFile = "../../core/controlplane/config/templates/cloud-config-etcd"
				stackTemplateOptions.RootStackTemplateTmplFile = "../../core/root/config/templates/stack-template.json"
				stackTemplateOptions.NodePoolStackTemplateTmplFile = "../../core/nodepool/config/templates/stack-template.json"
				stackTemplateOptions.ControlPlaneStackTemplateTmplFile = "../../core/controlplane/config/templates/stack-template.json"

				helper.WithPlugins(validCase.plugins, func() {
					cluster, err := root.ClusterFromConfig(providedConfig, stackTemplateOptions, false)
					if err != nil {
						t.Errorf("failed to create cluster driver : %v", err)
						t.FailNow()
					}

					t.Run("AssertCluster", func(t *testing.T) {
						for _, assertion := range validCase.assertCluster {
							assertion(cluster, t)
						}
					})

					t.Run("ValidateTemplates", func(t *testing.T) {
						if err := cluster.ValidateTemplates(); err != nil {
							t.Errorf("failed to render stack template: %v", err)
						}
					})

					if os.Getenv("KUBE_AWS_INTEGRATION_TEST") == "" {
						t.Skipf("`export KUBE_AWS_INTEGRATION_TEST=1` is required to run integration tests. Skipping.")
						t.SkipNow()
					} else {
						t.Run("ValidateStack", func(t *testing.T) {
							if !s3URIExists {
								t.Errorf("failed to obtain value for KUBE_AWS_S3_DIR_URI")
								t.FailNow()
							}

							report, err := cluster.ValidateStack()

							if err != nil {
								t.Errorf("failed to validate stack: %s %v", report, err)
							}
						})
					}
				})
			})
		})
	}
}
