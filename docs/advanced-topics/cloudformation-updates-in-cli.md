# CloudFormation Streaming

CloudFormation stack events can be streamed to the console (stdout) during a `kube-aws up` or `kube-aws update` operation.

The format of the messages are:
```
TimePassed   StackName   ResourceStatus    LogicalResourceId   StatusReason
```
For example:
```
+00:02:45	Nodepoolb	CREATE_IN_PROGRESS      		WorkersLC             	"Resource creation Initiated"
+00:02:45	Nodepoolb	CREATE_COMPLETE         		WorkersLC
+00:02:48	Nodepoolb	CREATE_IN_PROGRESS      		Workers
```

This feature is enabled by default and configurable in cluster.yaml:

```
cloudFormationStreaming: true
```