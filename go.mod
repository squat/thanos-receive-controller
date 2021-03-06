module github.com/observatorium/thanos-receive-controller

go 1.12

replace (
	github.com/improbable-eng/thanos => github.com/squat/thanos v0.5.0-rc.0.0.20190617145307-251a6da0d967
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190615125933-7de88b14dcc8
)

require (
	github.com/go-kit/kit v0.8.0
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/improbable-eng/thanos v0.5.0
	github.com/oklog/run v1.0.0
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8 // indirect
	golang.org/x/net v0.0.0-20190613194153-d28f0bde5980 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190614205929-e4e27c96b39a
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v2.0.0-alpha.0.0.20181121191925-a47917edff34+incompatible
)
