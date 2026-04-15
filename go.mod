module github.com/mgt-tool/mgtt-provider-kubernetes

go 1.25.7

// Pin to the local checkout until mgtt v0.1.0 is tagged upstream; remove
// this line after the tag lands.
replace github.com/mgt-tool/mgtt => /root/docs/projects/mgtt

require github.com/mgt-tool/mgtt v0.0.0-00010101000000-000000000000
