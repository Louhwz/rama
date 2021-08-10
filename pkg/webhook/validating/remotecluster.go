package validating

import (
	"context"
	"net/http"
	"regexp"

	ramav1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	r                = regexp.MustCompile(`^(https?://)[\w-]+(\.[\w-]+)+:\d{1,5}$`)
	remoteClusterGVK = gvkConverter(ramav1.SchemeGroupVersion.WithKind("RemoteCluster"))
)

func init() {
	createHandlers[remoteClusterGVK] = RCCreateValidation
	updateHandlers[remoteClusterGVK] = RCUpdateValidation
	deleteHandlers[remoteClusterGVK] = RCDeleteValidation
}

func RCCreateValidation(ctx context.Context, req *admission.Request, handler *Handler) admission.Response {
	rc := &ramav1.RemoteCluster{}
	err := handler.Decoder.Decode(*req, rc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	return validate(ctx, rc)
}

func RCUpdateValidation(ctx context.Context, req *admission.Request, handler *Handler) admission.Response {
	var (
		err   error
		newRC *ramav1.RemoteCluster
	)
	if err = handler.Decoder.DecodeRaw(req.Object, newRC); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	return validate(ctx, newRC)
}

func RCDeleteValidation(ctx context.Context, req *admission.Request, handler *Handler) admission.Response {
	return admission.Allowed("validation pass")
}

func validate(ctx context.Context, rc *ramav1.RemoteCluster) admission.Response {
	connConfig := rc.Spec.ConnConfig
	if connConfig.Endpoint == "" || connConfig.CABundle == nil || connConfig.ClientKey == nil || connConfig.ClientCert == nil {
		return admission.Denied("empty connection config, please check.")
	}

	if !r.Match([]byte(connConfig.Endpoint)) {
		return admission.Denied("endpoint format: https://server:address")
	}
	return admission.Allowed("validation pass")
}
