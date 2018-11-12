package api

// HostedZone is a AWS Route 53 hosted zone in which record sets are created.
// Record sts are created to register DNS records to make various DNS names of nodes and/or load LBs managed by kube-aws
// visible to an internal network or the internet
type HostedZone struct {
	// Identifier should include the hosted zone ID for a private or public hosted zone,
	// to make DNS names available to an internal network or the internet respectively
	Identifier `yaml:",inline"`
}
