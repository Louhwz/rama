package validating

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sync"

	ramav1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/utils"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	r                = regexp.MustCompile(`^(https?://)[\w-]+(\.[\w-]+)+:\d{1,5}$`)
	mu               sync.Mutex
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
	mu.Lock()
	defer mu.Unlock()

	connConfig := rc.Spec.ConnConfig
	if connConfig.Endpoint == "" || connConfig.CABundle == nil || connConfig.ClientKey == nil || connConfig.ClientCert == nil {
		return admission.Denied("empty connection config, please check.")
	}
	if !r.Match([]byte(connConfig.Endpoint)) {
		return admission.Denied("endpoint format: https://server:address, please check")
	}

	cfg, err := utils.BuildClusterConfig(rc)
	if err != nil {
		return admission.Denied(fmt.Sprintf("Can't build connection to remote cluster, maybe wrong config. Err=%v", err.Error()))
	}
	coreClient := v1.NewForConfigOrDie(cfg)
	uuid, err := utils.GetUUID(coreClient)
	if err != nil {
		return admission.Denied(fmt.Sprintf("Can't get uuid. Err=%v", err))
	}

	rcs, err := RCLister.List(labels.NewSelector())
	if err != nil {
		return admission.Denied(fmt.Sprintf("Can't list remote cluster. Err=%v", err))
	}
	for _, rc := range rcs {
		if uuid == rc.Status.UUID {
			return admission.Denied("Duplicate cluster configuration")
		}
	}
	return admission.Allowed("validation pass")
}
