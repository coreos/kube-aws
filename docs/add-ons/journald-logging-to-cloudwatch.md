# Journald logging to AWS CloudWatch

A service has been introduced which runs a dockerised image of *journald-cloudwatch-logs*. This service forwards journald logs to AWS CloudWatch to a LogGroup with the name of .ClusterName and is run on all nodes (Etcds, Controllers and Workers).

*journald-cloudwatch-logs* is a goLang project https://github.com/saymedia/journald-cloudwatch-logs.

The default docker image *[jollinshead/journald-cloudwatch-logs](https://hub.docker.com/r/jollinshead/journald-cloudwatch-logs/)* is a wrapper around the go binary of *journald-cloudwatch-logs*.

This feature is disabled by default and configurable in cluster.yaml:

```yaml
cloudWatchLogging:
  enabled: false
  retentionInDays: 7
```


The docker image is also configurable:

```yaml
journaldCloudWatchLogsImage:
  repo: "jollinshead/journald-cloudwatch-logs"
  tag: "0.1"
  rktPullDocker: true
```

## kube-aws up/update feedback

During kube-aws up/update, filtered Journald logs can be printed to stdout. This may assist debugging.
The format of the messages are:
```
TimePassed   NodeName: "LogMessage"
```
For example:
```
+00:04:51	ip-10-29-29-100.us-west-2.compute.internal: "check-certification-validity.service: Failed to run 'start-pre' task: No such file or directory"
+00:04:52	ip-10-29-29-100.us-west-2.compute.internal: "check-certification-validity.service: Failed with result 'resources'."
+00:04:53	ip-10-29-29-100.us-west-2.compute.internal: "kubelet.service: Failed with result 'exit-code'."
```

This feature is configurable in cluster.yaml under the *cloudWatchLogging* section, and requires *cloudWatchLogging* to be enabled.
( Default values: )

```yaml
cloudWatchLogging:
  enabled: false
  imageWithTag: jollinshead/journald-cloudwatch-logs:0.1
  retentionInDays: 7
  localStreaming:
    enabled: true
    filter:  `{ $.priority = "CRIT" || $.priority = "WARNING" && $.transport = "journal" && $.systemdUnit = "init.scope" }`
    interval: 60
```

NOTE: Due to high initial entropy, *.service* failures may occur during the early stages of booting.
In this context Entropy refers to the disorder of *.service*s (starting, failing, restarting).

### Parameters

#### Filter
By default the filter is configured for *.service* failures and messages flagged as 'critical'.
See [the official AWS documentation](http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html) for more information.

#### Interval
Since some messages are produced frequently, to avoid excessive spam, an 'interval' parameter is provided.
This 'interval' value determines the time between printing two identical messages to stdout.

