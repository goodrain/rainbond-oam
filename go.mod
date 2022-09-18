module github.com/goodrain/rainbond-oam

go 1.13

require (
	github.com/Microsoft/hcsshim v0.9.4 // indirect
	github.com/containerd/containerd v1.5.7
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/crossplane/oam-kubernetes-runtime v0.1.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/google/uuid v1.2.0
	github.com/mozillazg/go-pinyin v0.18.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pquerna/ffjson v0.0.0-20190930134022-aa0246cd15f7
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6

)

replace k8s.io/api v0.20.4 => k8s.io/api v0.20.6
