module knative.dev/operator

go 1.16

require (
	github.com/go-logr/zapr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/google/go-github/v33 v33.0.0
	github.com/manifestival/client-go-client v0.5.0
	github.com/manifestival/manifestival v0.7.0
	go.uber.org/zap v1.16.0
	gocloud.dev v0.22.0
	golang.org/x/mod v0.4.1
	golang.org/x/oauth2 v0.0.0-20210413134643-5e61552d6c78
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	knative.dev/caching v0.0.0-20210525133838-9c0829407b5a
	knative.dev/eventing v0.23.1-0.20210524085620-e48c80d7f969
	knative.dev/hack v0.0.0-20210428122153-93ad9129c268
	knative.dev/pkg v0.0.0-20210525071438-e2947dba201a
	knative.dev/serving v0.23.1-0.20210525132337-efbc518aa54e
	sigs.k8s.io/yaml v1.2.0
)
