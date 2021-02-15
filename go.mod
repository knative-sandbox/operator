module knative.dev/operator

go 1.15

require (
	github.com/go-logr/zapr v0.4.0
	github.com/google/go-cmp v0.5.4
	github.com/google/go-github/v33 v33.0.0
	github.com/manifestival/client-go-client v0.4.0
	github.com/manifestival/manifestival v0.6.1
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.4.1
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	knative.dev/caching v0.0.0-20210215030244-1212288570f0
	knative.dev/eventing v0.20.1-0.20210215082944-d8468ca728b3
	knative.dev/hack v0.0.0-20210203173706-8368e1f6eacf
	knative.dev/pkg v0.0.0-20210212203835-448ae657fb5f
	knative.dev/serving v0.20.1-0.20210215125429-395bda90779d
	sigs.k8s.io/yaml v1.2.0
)
